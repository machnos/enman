package main

import (
	"context"
	"enman/internal/config"
	"enman/internal/controllers"
	"enman/internal/domain"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/http"
	"enman/internal/log"
	"enman/internal/meters"
	"enman/internal/modbus"
	"enman/internal/modbus/server"
	"enman/internal/persistency/noop"
	"enman/internal/persistency/timescale"
	"enman/internal/price_importers"
	"enman/internal/price_importers/energyzero"
	"enman/internal/price_importers/entsoe"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	log.ActiveLevel = log.LvlInfo
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	syncGroup, syncGroupContext := errgroup.WithContext(ctx)

	// Parse command line parameters.
	configFile := flag.String("config-file", "enman.yaml", "Full path to the configuration file")
	flag.Parse()

	// Load configuration
	configuration, err := config.LoadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Unable to load configuration file: %s", err.Error())
		syscall.Exit(-1)
		return
	}
	if configuration.Log != nil && configuration.Log.Level != 0 {
		log.ActiveLevel = log.Level(configuration.Log.Level)
	}

	// Initialize package-specific log levels
	if configuration.Log != nil && configuration.Log.Packages != nil {
		packageLevels := make(map[string]log.Level)
		for _, pkg := range configuration.Log.Packages {
			packageLevels[pkg.Name] = log.Level(pkg.Level)
		}
		log.SetPackageLevels(packageLevels)
	}

	// Setup system
	system := domain.NewSystem(time.Now().Location())
	energyMeters, err := meters.ProbeEnergyMeters(constants.EnergySourceRoleGrid, configuration.Grid.Meters)
	if err != nil {
		log.Fatalf("unable to probe grid meter: %s", err.Error())
		syscall.Exit(-1)
	}
	system.SetGrid(configuration.Grid.Name,
		configuration.Grid.Voltage,
		configuration.Grid.MaxCurrent,
		configuration.Grid.Phases,
		configuration.Grid.ElectricityTargetConsumption,
		energyMeters,
		controllers.ProbeGridController(configuration.Grid.Controller),
	)
	var pvStateController *domain.PvStateController
	if configuration.Pvs != nil {
		for _, pv := range configuration.Pvs.Arrays {
			energyMeters, err = meters.ProbeEnergyMeters(constants.EnergySourceRolePv, pv.Meters)
			if err != nil {
				log.Fatalf("unable to probe pv meter: %s", err.Error())
				syscall.Exit(-1)
			}
			system.AddPv(pv.Name, energyMeters, controllers.ProbePvController(pv.Controller))
		}
		if configuration.Pvs.PvStateController != nil {
			pvStateController = domain.NewPvStateController(
				system,
				configuration.Pvs.PvStateController.DisableFormula,
				float32(configuration.Pvs.PvStateController.BatteryCutoffPercentage),
				float32(configuration.Pvs.PvStateController.BatteryRestartPercentage),
			)
		}
	}
	for _, acLoad := range configuration.AcLoads {
		energyMeters, err = meters.ProbeEnergyMeters(constants.EnergySourceRole(acLoad.Role), acLoad.Meters)
		if err != nil {
			log.Fatalf("unable to probe ac load meter: %s", err.Error())
			syscall.Exit(-1)
		}
		system.AddAcLoad(acLoad.Name,
			constants.EnergySourceRole(acLoad.Role),
			acLoad.PercentageFromGrid,
			energyMeters,
		)
	}
	for _, battery := range configuration.Batteries.Batteries {
		energyMeters, err = meters.ProbeEnergyMeters(constants.EnergySourceRoleBattery, battery.Meters)
		if err != nil {
			log.Fatalf("unable to probe battery meter: %s", err.Error())
			syscall.Exit(-1)
		}
		system.AddBattery(domain.NewBattery(battery.Name,
			battery.Capacity,
			battery.ChargingCoefficient,
			battery.DischargingCoefficient,
			battery.Voltage,
			battery.RoundTripEfficiency,
			energyMeters,
		))
	}

	// Setup repository
	repo := loadRepository(configuration)
	err = repo.Initialize()
	if err != nil {
		log.Fatalf("Unable to initialize database: %s", err.Error())
		syscall.Exit(-1)
	}

	// Setup domain event listeners
	electricityCostCalculator := domain.NewElectricityUsageCostCalculator(repo)
	events.EnergyPrices.Register(electricityCostCalculator, func(values *events.EnergyPriceValues) bool {
		return values.EnergyType() == prices.EnergyTypeElectricity
	})
	gasCostCalculator := domain.NewGasUsageCostCalculator(repo)
	events.EnergyPrices.Register(gasCostCalculator, func(values *events.EnergyPriceValues) bool {
		return values.EnergyType() == prices.EnergyTypeGas
	})

	// Setup grid target consumption calculator
	peakPriceStdDevMultiplier := float32(1.0)
	if configuration.Batteries.PeakPriceDetectionStandardDeviationMultiplier > 0 {
		peakPriceStdDevMultiplier = configuration.Batteries.PeakPriceDetectionStandardDeviationMultiplier
	}
	survivalSocThreshold := float32(25.0)
	if configuration.Batteries.SurvivalChargingSocThreshold > 0 && configuration.Batteries.SurvivalChargingSocThreshold <= 100 {
		survivalSocThreshold = configuration.Batteries.SurvivalChargingSocThreshold
	}
	gridTargetConsumptionCalculator, err := domain.NewGridTargetConsumptionCalculator(system, repo, peakPriceStdDevMultiplier, survivalSocThreshold)
	if err != nil {
		log.Warningf("Unable to start grid target consumption calculator: %s", err.Error())
	}

	// Set price importers
	if configuration.Prices != nil {
		baseImporter := &price_importers.BasePriceImporter{
			Repository:      repo,
			EnergyProviders: configuration.Prices.Providers,
		}
		rootImporters := make([]price_importers.PriceImporter, 0)
		if configuration.Prices.Entsoe != nil && configuration.Prices.Entsoe.SecurityToken != "" {
			importer, err := entsoe.NewEntsoeImporter(
				baseImporter,
				configuration.Prices.Country,
				configuration.Prices.Area,
				configuration.Prices.Entsoe.SecurityToken,
			)
			if err != nil {
				log.Error(err.Error())
				return
			}
			rootImporters = append(rootImporters, importer)
		}
		if configuration.Prices.EnergyZero {
			importer := energyzero.NewEnergyZeroImporter(baseImporter)
			rootImporters = append(rootImporters, importer)
		}
		tNow := time.Now()
		year, month, day := tNow.Date()
		startOfToday := time.Date(year, month, day, 0, 0, 0, 0, tNow.Location())
		endOfTomorrow := startOfToday.AddDate(0, 0, 2).Add(time.Nanosecond * -1)
		go func() {
			rootPrices := make([]*prices.EnergyPrice, 0)
			for _, rootImporter := range rootImporters {
				p, err := rootImporter.ImportPrices(ctx, startOfToday, endOfTomorrow)
				if err != nil {
					log.Errorf("Failed to import prices: %v", err.Error())
				} else {
					rootPrices = append(rootPrices, p...)
				}
			}
			baseImporter.UpdateProviderPrices(syncGroupContext, rootPrices)
		}()
		ticker := time.NewTicker(1 * time.Hour)
		go func() {
			for {
				select {
				case <-syncGroupContext.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					tNow = time.Now()
					year, month, day = tNow.Date()
					startOfToday = time.Date(year, month, day, 0, 0, 0, 0, tNow.Location())
					endOfTomorrow = startOfToday.AddDate(0, 0, 2).Add(time.Nanosecond * -1)
					rootPrices := make([]*prices.EnergyPrice, 0)
					for _, rootImporter := range rootImporters {
						p, err := rootImporter.ImportPrices(ctx, startOfToday, endOfTomorrow)
						if err != nil {
							log.Errorf("Failed to import prices: %v", err.Error())
						} else {
							rootPrices = append(rootPrices, p...)
						}
					}
					baseImporter.UpdateProviderPrices(syncGroupContext, rootPrices)
				}
			}
		}()
	}

	modbusServers, _ := createModbusServers(configuration, system)
	for _, modbusServer := range modbusServers {
		syncGroup.Go(func() error {
			log.Infof("Starting modbus server on %s", modbusServer.ServerUrl())
			err = modbusServer.Start()
			if err != nil {
				log.Errorf("Failed to start modbus server server: %s", err.Error())
				return err
			}
			log.Infof("Modbus server on %s started", modbusServer.ServerUrl())
			return nil
		})
	}
	// Start all meters on the System.
	system.StartMeasuring(syncGroupContext)
	// Start the PV state controller
	if pvStateController != nil {
		pvStateController.Start(syncGroupContext)
	}

	// Start the http server
	httpServer, err := http.NewServer(configuration.Http, system, repo)
	if err != nil {
		log.Warningf("Failed to create http server: %s", err.Error())
	}
	syncGroup.Go(func() error {
		return httpServer.Start()
	})
	syncGroup.Go(func() error {
		<-syncGroupContext.Done()
		if httpServer != nil {
			err = httpServer.Shutdown(context.Background())
			if err != nil {
				log.Warningf("Failed to stop http server: %s", err.Error())
			}
		}
		if electricityCostCalculator != nil {
			events.EnergyPrices.Deregister(electricityCostCalculator)
		}
		if gasCostCalculator != nil {
			events.EnergyPrices.Deregister(gasCostCalculator)
		}
		if gridTargetConsumptionCalculator != nil {
			gridTargetConsumptionCalculator.Stop()
		}
		modbus.EmptyClientCache()
		if modbusServers != nil {
			for _, modbusServer := range modbusServers {
				log.Infof("Shutting down modbus server on %s", modbusServer.ServerUrl())
				err := modbusServer.Stop()
				if err != nil {
					log.Warningf("Failed to stop modbus server: %s", err.Error())
				}
				log.Infof("Modbus server on %s shutdown", modbusServer.ServerUrl())
			}
		}
		if repo != nil {
			repo.Close()
		}
		return nil
	})
	if err = syncGroup.Wait(); err != nil {
		log.Errorf("%v", err)
	}
}

func loadRepository(configuration *config.Configuration) repository.Repository {
	if configuration.Persistency != nil {
		if configuration.Persistency.Timescale != nil {
			repo, err := timescale.NewTimescaleRepository(configuration.Persistency.Timescale.ConnectionString)
			if err == nil {
				return repo
			}
			log.Warningf("Unable to create timescale repository: %s", err.Error())
		}
	}
	log.Warning("Persistency not or not correctly configured. Energy measurements will not be stored!")
	return noop.NewNoopRepository()
}

func createModbusServers(config *config.Configuration, system *domain.System) ([]*modbus.ModbusServer, error) {
	if config.ModbusServers == nil {
		return nil, nil
	}
	var servers []*modbus.ModbusServer
	requestHandler := server.NewDispatchingRequestHandler(system)
	if system.Grid() != nil && config.Grid.ModbusMeterSimulator != nil {
		simulator, err := server.NewMeterSimulator(
			config.Grid.ModbusMeterSimulator.MeterType,
			config.Grid.ModbusMeterSimulator.ModbusUnitId,
			system.Grid().ElectricityState(),
			system.Grid().ElectricityUsage())
		if err != nil {
			return nil, err
		}
		log.Infof("Adding %s energy meter simulator for Grid %s at unit id %d", config.Grid.ModbusMeterSimulator.MeterType, config.Grid.Name, config.Grid.ModbusMeterSimulator.ModbusUnitId)
		requestHandler.AddHandler(config.Grid.ModbusMeterSimulator.ModbusUnitId, simulator)
	}
	if config.Pvs != nil {
		for _, pv := range config.Pvs.Arrays {
			if pv.ModbusMeterSimulator != nil {
				if pv.Name == "" {
					log.Warningf("PV with modbus simulator id %d has no name. The name is required for the meter simulator to work. ", pv.ModbusMeterSimulator.ModbusUnitId)
					continue
				}
				for _, p := range system.Pvs() {
					if p.Name() == pv.Name {
						simulator, err := server.NewMeterSimulator(
							pv.ModbusMeterSimulator.MeterType,
							pv.ModbusMeterSimulator.ModbusUnitId,
							p.State(),
							p.Usage())
						if err != nil {
							return nil, err
						}
						log.Infof("Adding %s energy meter simulator for Pv %s at unit id %d", pv.ModbusMeterSimulator.MeterType, pv.Name, pv.ModbusMeterSimulator.ModbusUnitId)
						requestHandler.AddHandler(pv.ModbusMeterSimulator.ModbusUnitId, simulator)
						break
					}
				}
			}
		}
	}
	for _, acLoad := range config.AcLoads {
		if acLoad.ModbusMeterSimulator != nil {
			if acLoad.Name == "" {
				log.Warningf("AcLoad with modbus simulator id %d has no name. The name is required for the meter simulator to work. ", acLoad.ModbusMeterSimulator.ModbusUnitId)
				continue
			}
			for _, a := range system.AcLoads() {
				if a.Name() == acLoad.Name && a.Role() == constants.EnergySourceRole(acLoad.Role) {
					simulator, err := server.NewMeterSimulator(
						acLoad.ModbusMeterSimulator.MeterType,
						acLoad.ModbusMeterSimulator.ModbusUnitId,
						a.State(),
						a.Usage())
					if err != nil {
						return nil, err
					}
					log.Infof("Adding %s energy meter simulator for AcLoad %s at unit id %d", acLoad.ModbusMeterSimulator.MeterType, acLoad.Name, acLoad.ModbusMeterSimulator.ModbusUnitId)
					requestHandler.AddHandler(acLoad.ModbusMeterSimulator.ModbusUnitId, simulator)
					break
				}
			}
		}
	}

	for _, modbusProxy := range config.ModbusServers {
		modbusServer, err := modbus.NewServer(&modbus.ServerConfiguration{
			URL:        modbusProxy.ServerUrl,
			Speed:      uint(modbusProxy.Speed),
			DataBits:   uint(modbusProxy.DataBits),
			Parity:     uint(modbusProxy.Parity),
			StopBits:   uint(modbusProxy.StopBits),
			MaxClients: uint(modbusProxy.MaxClients),
		}, requestHandler)
		if err != nil {
			log.Errorf("Failed to create modbus server server: %s", err.Error())
			continue
		}
		servers = append(servers, modbusServer)
	}
	return servers, nil
}

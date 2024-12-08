package main

import (
	"context"
	"enman/internal/config"
	"enman/internal/controllers"
	"enman/internal/domain"
	"enman/internal/http"
	"enman/internal/log"
	"enman/internal/meters"
	"enman/internal/modbus"
	"enman/internal/modbus/server"
	"enman/internal/persistency/influx"
	"enman/internal/persistency/noop"
	"enman/internal/prices/entsoe"
	"flag"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	}
	if configuration.Log != nil && configuration.Log.Level != 0 {
		log.ActiveLevel = log.Level(configuration.Log.Level)
	}

	// Setup system
	system := domain.NewSystem(time.Now().Location())
	system.SetGrid(configuration.Grid.Name,
		configuration.Grid.Voltage,
		configuration.Grid.MaxCurrent,
		configuration.Grid.Phases,
		configuration.Grid.TargetConsumption,
		meters.ProbeEnergyMeters(domain.RoleGrid, configuration.Grid.Meters),
		controllers.ProbeGridController(configuration.Grid.Controller),
	)
	var pvStateController *domain.PvStateController
	if configuration.Pvs != nil {
		for _, pv := range configuration.Pvs.Arrays {
			system.AddPv(pv.Name, meters.ProbeEnergyMeters(domain.RolePv, pv.Meters), controllers.ProbePvController(pv.Controller))
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
		system.AddAcLoad(acLoad.Name,
			domain.EnergySourceRole(acLoad.Role),
			acLoad.PercentageFromGrid,
			meters.ProbeEnergyMeters(domain.EnergySourceRole(acLoad.Role), acLoad.Meters),
		)
	}
	for _, battery := range configuration.Batteries {
		system.AddBattery(battery.Name,
			meters.ProbeEnergyMeters(domain.RoleBattery, battery.Meters),
		)
	}

	// Setup repository
	repository := loadRepository(configuration)
	err = repository.Initialize()
	if err != nil {
		log.Warningf("Unable to initialize database: %s", err.Error())
		//syscall.Exit(-1)
	}

	// Setup domain event listeners
	costCalculator := domain.NewElectricityUsageCostCalculator(repository)
	domain.ElectricityPrices.Register(costCalculator, nil)

	// Setup grid target consumption calculator
	gridTargetConsumptionCalculator, err := domain.NewGridTargetConsumptionCalculator(system)
	if err != nil {
		log.Warningf("Unable to start grid target consumption calculator: %s", err.Error())
	}

	// Set price importers
	if configuration.Prices != nil {
		importer, err := entsoe.NewEntsoeImporter(
			configuration.Prices.Country,
			configuration.Prices.Area,
			configuration.Prices.Entsoe.SecurityToken,
			configuration.Prices.Providers,
			repository,
		)
		if err != nil {
			log.Error(err.Error())
			return
		}
		t := time.Now()
		year, month, day := t.Date()
		start := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
		go func() {
			err = importer.ImportPrices(ctx, start, start.AddDate(0, 0, 2).Add(time.Nanosecond*-1))
			if err != nil {
				log.Errorf("Failed to import prices: %v", err.Error())
			}
		}()
		ticker := time.NewTicker(1 * time.Hour)
		go func() {
			for {
				select {
				case <-syncGroupContext.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					t = time.Now()
					year, month, day = t.Date()
					start = time.Date(year, month, day, 0, 0, 0, 0, t.Location())
					err = importer.ImportPrices(ctx, start, start.AddDate(0, 0, 2).Add(time.Nanosecond*-1))
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		}()
	}

	modbusServers, _ := createModbusServers(configuration, system)
	for _, server := range modbusServers {
		syncGroup.Go(func() error {
			log.Infof("Starting modbus server on %s", server.ServerUrl())
			err = server.Start()
			if err != nil {
				log.Errorf("Failed to start modbus server server: %s", err.Error())
				return err
			}
			log.Infof("Modbus server on %s started", server.ServerUrl())
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
	httpServer, err := http.NewServer(configuration.Http, system, repository)
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
		if costCalculator != nil {
			domain.ElectricityPrices.Deregister(costCalculator)
		}
		if gridTargetConsumptionCalculator != nil {
			gridTargetConsumptionCalculator.Stop()
		}
		modbus.EmptyClientCache()
		if modbusServers != nil {
			for _, server := range modbusServers {
				log.Infof("Shutting down modbus server on %s", server.ServerUrl())
				err := server.Stop()
				if err != nil {
					log.Warningf("Failed to stop modbus server: %s", err.Error())
				}
				log.Infof("Modbus server on %s shutdown", server.ServerUrl())
			}
		}
		if repository != nil {
			repository.Close()
		}
		return nil
	})
	if err = syncGroup.Wait(); err != nil {
		log.Errorf("%v", err)
	}
}

func loadRepository(configuration *config.Configuration) domain.Repository {
	var repository domain.Repository
	if configuration.Persistency != nil {
		if configuration.Persistency.Influx != nil {
			influxConfig := configuration.Persistency.Influx
			repository = influx.NewInfluxRepository(influxConfig.ServerUrl, influxConfig.Token)
		}
	}
	if repository == nil {
		log.Warning("Persistency not configured. Energy measurements will not be stored.")
		repository = noop.NewNoopRepository()
	}
	return repository
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
							p.ElectricityState(),
							p.ElectricityUsage())
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
				if a.Name() == acLoad.Name && a.Role() == domain.EnergySourceRole(acLoad.Role) {
					simulator, err := server.NewMeterSimulator(
						acLoad.ModbusMeterSimulator.MeterType,
						acLoad.ModbusMeterSimulator.ModbusUnitId,
						a.ElectricityState(),
						a.ElectricityUsage())
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
		server, err := modbus.NewServer(&modbus.ServerConfiguration{
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
		servers = append(servers, server)
	}
	return servers, nil
}

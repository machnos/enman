package main

import (
	"context"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/http"
	"enman/internal/log"
	"enman/internal/meters"
	"enman/internal/modbus"
	"enman/internal/modbus/proxy"
	"enman/internal/persistency/influx"
	"enman/internal/persistency/noop"
	"enman/internal/prices/entsoe"
	"flag"
	"fmt"
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
	configFile := *flag.String("config-file", "config.json", "Full path to the configuration file")
	flag.Parse()

	// Load configuration
	configuration, err := config.LoadConfiguration(configFile)
	if err != nil {
		log.Fatalf("Unable to load configuration file: %s", err.Error())
		syscall.Exit(-1)
	}
	if configuration.LogLevel != 0 {
		log.ActiveLevel = log.Level(configuration.LogLevel)
	}

	// Setup system
	system := domain.NewSystem(time.Now().Location())
	system.SetGrid(configuration.Grid.Name, configuration.Grid.Voltage, configuration.Grid.MaxCurrent, configuration.Grid.Phases)
	for _, pv := range configuration.Pvs {
		system.AddPv(pv.Name)
	}

	// Setup repository
	repository := loadRepository(configuration)
	err = repository.Initialize()
	if err != nil {
		log.Warningf("Unable to initialize database: %s", err.Error())
		//syscall.Exit(-1)
	}
	syncGroup.Go(func() error {
		<-syncGroupContext.Done()
		repository.Close()
		return nil
	})

	// Setup domain event listeners
	costCalculator := domain.NewElectricityUsageCostCalculator(repository)
	domain.ElectricityPrices.Register(costCalculator, nil)
	syncGroup.Go(func() error {
		<-syncGroupContext.Done()
		domain.ElectricityPrices.Deregister(costCalculator)
		return nil
	})

	// Setup modbus/serial electricity meters
	createElectricityMeter(configuration.Grid.Name, domain.RoleGrid, configuration.Grid.Meters).StartReading(syncGroupContext)
	for _, pv := range configuration.Pvs {
		createElectricityMeter(pv.Name, domain.RolePv, pv.Meters).StartReading(syncGroupContext)
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
				log.Error(err.Error())
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

	modbusServers := createModbusServers(configuration, system)
	for _, server := range modbusServers {
		syncGroup.Go(func() error {
			log.Infof("Starting modbus proxy on %s", server.ServerUrl())
			err = server.Start()
			if err != nil {
				log.Errorf("Failed to start modbus proxy server: %s", err.Error())
				return err
			}
			log.Infof("Modbus proxy on %s started", server.ServerUrl())
			return nil
		})
		syncGroup.Go(func() error {
			<-syncGroupContext.Done()
			log.Infof("Shutting down modbus proxy on %s", server.ServerUrl())
			err := server.Stop()
			if err != nil {
				log.Warningf("Failed to stop modbus proxy on: %s", err.Error())
				return err
			}
			log.Infof("Modbus proxy on %s shutdown", server.ServerUrl())
			return nil
		})
	}

	httpServer, err := http.NewServer(configuration.Http, system, repository)
	if err != nil {
		log.Warningf("Failed to create http server: %s", err.Error())
	}
	syncGroup.Go(func() error {
		return httpServer.Start()
	})
	syncGroup.Go(func() error {
		<-syncGroupContext.Done()
		return httpServer.Shutdown(context.Background())
	})
	if err := syncGroup.Wait(); err != nil {
		log.Errorf("%v", err)
	}
}

func createElectricityMeter(name string, role domain.ElectricitySourceRole, meterConfigs []*config.ElectricityMeter) domain.ElectricityMeter {
	if meterConfigs == nil || len(meterConfigs) < 1 {
		return nil
	}
	if domain.RoleGrid == role && len(meterConfigs) == 1 && meterConfigs[0].Type == "serial" {
		return meters.NewElectricitySerialMeter(
			name,
			role,
			meterConfigs[0].Brand,
			meterConfigs[0].Speed,
			meterConfigs[0].ConnectURL,
			meterConfigs[0].LineIndices,
			meterConfigs[0].Attributes)
	}
	if len(meterConfigs) > 1 {
		singleMeters := make([]domain.ElectricityMeter, 0)
		for ix, meterConfig := range meterConfigs {
			singleMeters = append(singleMeters, meters.NewElectricityModbusMeter(
				fmt.Sprintf("%s-%d", name, ix+1),
				role,
				meterConfig.Brand,
				meterConfig.Speed,
				meterConfig.ConnectURL,
				meterConfig.ModbusUnitId,
				meterConfig.LineIndices,
				meterConfig.Attributes))
		}
		return meters.NewCompoundElectricityMeter(name, role, singleMeters)
	} else {
		return meters.NewElectricityModbusMeter(
			name,
			role,
			meterConfigs[0].Brand,
			meterConfigs[0].Speed,
			meterConfigs[0].ConnectURL,
			meterConfigs[0].ModbusUnitId,
			meterConfigs[0].LineIndices,
			meterConfigs[0].Attributes)
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

func createModbusServers(config *config.Configuration, system *domain.System) []*modbus.ModbusServer {
	if config.ModbusServers == nil {
		return nil
	}
	var servers []*modbus.ModbusServer
	requestHandler := proxy.NewDispatchingRequestHandler()
	if system.Grid() != nil && config.Grid.ModbusMeterSimulator != nil {
		simulator := proxy.NewMeterSimulator(
			config.Grid.ModbusMeterSimulator.MeterType,
			config.Grid.ModbusMeterSimulator.ModbusUnitId,
			system.Grid().ElectricityState(),
			system.Grid().ElectricityUsage())
		if simulator != nil {
			log.Infof("Adding %s energy meter simulator for Grid %s at unit id %d", config.Grid.ModbusMeterSimulator.MeterType, config.Grid.Name, config.Grid.ModbusMeterSimulator.ModbusUnitId)
			requestHandler.AddHandler(config.Grid.ModbusMeterSimulator.ModbusUnitId, simulator)
		}
	}
	for _, pv := range config.Pvs {
		if pv.ModbusMeterSimulator != nil {
			if pv.Name == "" {
				log.Warningf("PV with modbus simulator id %d has no name. The name is required for the meter simulator to work. ", pv.ModbusMeterSimulator.ModbusUnitId)
				continue
			}
			for _, p := range system.Pvs() {
				if p.Name() == pv.Name {
					log.Infof("Adding %s energy meter simulator for Pv %s at unit id %d", pv.ModbusMeterSimulator.MeterType, pv.Name, pv.ModbusMeterSimulator.ModbusUnitId)
					simulator := proxy.NewMeterSimulator(
						pv.ModbusMeterSimulator.MeterType,
						pv.ModbusMeterSimulator.ModbusUnitId,
						p.ElectricityState(),
						p.ElectricityUsage())
					if simulator != nil {
						requestHandler.AddHandler(pv.ModbusMeterSimulator.ModbusUnitId, simulator)
					}
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
			log.Errorf("Failed to create modbus proxy server: %s", err.Error())
			continue
		}
		servers = append(servers, server)
	}
	return servers
}

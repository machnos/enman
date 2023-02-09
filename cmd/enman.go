package main

import (
	"encoding/json"
	"enman/internal"
	"enman/internal/balance"
	"enman/internal/config"
	"enman/internal/energysource"
	"enman/internal/energysource/modbus"
	"enman/internal/energysource/modbus/abb"
	"enman/internal/energysource/modbus/carlo_gavazzi"
	"enman/internal/energysource/modbus/dsmr"
	"enman/internal/energysource/modbus/victron"
	"enman/internal/log"
	modbusProtocol "enman/internal/modbus"
	"enman/internal/modbus/proxy"
	"enman/internal/persistency"
	"flag"
	"fmt"
	"io"
	"net/http"
	"syscall"
)

type home struct {
	system            *internal.System
	modbusUpdateLoops map[string]*modbus.UpdateLoop
}

func (h *home) Close() {
	for _, value := range h.modbusUpdateLoops {
		value.Close()
	}
}

func (h *home) addModbusUpdateLoop(connectUrl string, updateChannels *internal.UpdateChannels, newModbusConfiguration func(string) *modbusProtocol.ClientConfiguration) {
	if h.modbusUpdateLoops[connectUrl] == nil {
		updateLoop, err := modbus.NewUpdateLoop(newModbusConfiguration(connectUrl), updateChannels)
		if err != nil {
			log.Fatalf("Unable to setup modbus connection to %s: %s", connectUrl, err.Error())
			panic(err)
		}
		h.modbusUpdateLoops[connectUrl] = updateLoop
	}
}

func main() {
	log.ActiveLevel = log.LvlInfo

	// Parse command line parameters.
	configFile := *flag.String("config-file", "config.json", "Full path to the configuration file")
	flag.Parse()

	// Load configuration
	configuration := config.LoadConfiguration(configFile)
	if configuration == nil {
		syscall.Exit(-1)
	}

	// Setup repository
	repository := loadRepository(configuration)
	defer repository.Close()

	h := &home{
		system:            &internal.System{},
		modbusUpdateLoops: make(map[string]*modbus.UpdateLoop),
	}
	defer h.Close()

	// Load grid & pv's from config.
	updateChannels := internal.NewUpdateChannels()
	addGrid(h, configuration.Grid, updateChannels)
	addPvs(h, configuration.Pvs, updateChannels)

	// Start the balancer update loop
	balance.StartUpdateLoop(updateChannels, repository)

	modbusServers := startModbusServer(configuration, h)
	for _, server := range modbusServers {
		defer func(server *modbusProtocol.ModbusServer) {
			err := server.Stop()
			if err != nil {
				log.Warningf("Failed to stop modbus server: %s", err.Error())
			}
		}(server)
	}

	// Start the http server
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.printStatusAsHtml)
	mux.HandleFunc("/api", h.dataAsJson)
	err := http.ListenAndServe(fmt.Sprintf(":%d", configuration.Http.Port), mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func loadRepository(configuration *config.Configuration) persistency.Repository {
	var repository persistency.Repository
	if configuration.Persistency != nil {
		if configuration.Persistency.Influx != nil {
			influx := configuration.Persistency.Influx
			repository = persistency.NewInfluxRepository(influx.ServerUrl, influx.Token)
		}
	}
	if repository == nil {
		log.Warning("Persistency not configured. Energy measurements will not be stored.")
		repository = persistency.NewNoopRepository()
	}
	return repository
}

func addGrid(h *home, grid *config.Grid, updateChannels *internal.UpdateChannels) {
	gridConfig, err := energysource.NewGridConfig(
		grid.Voltage,
		grid.MaxCurrent,
		grid.Phases,
	)
	if err != nil {
		log.Fatalf("Unable to create grid configuration: %s", err.Error())
		panic(err)
	}
	var g energysource.Grid
	switch grid.Brand {
	case config.ABB:
		g, err = createModbusGrid(h, grid, gridConfig, updateChannels, abb.NewModbusConfiguration, abb.NewGrid)
		if err != nil {
			log.Fatalf("Unable to create ABB grid: %s", err.Error())
			panic(err)
		}
	case config.CarloGavazzi:
		g, err = createModbusGrid(h, grid, gridConfig, updateChannels, carlo_gavazzi.NewModbusConfiguration, carlo_gavazzi.NewGrid)
		if err != nil {
			log.Fatalf("Unable to create Carlo Gavazzi grid: %s", err.Error())
			panic(err)
		}
	case config.DSMR:
		g, err = dsmr.NewDsmrGrid(grid.Name, &dsmr.DsmrConfig{
			BaudRate: 115200,
			Device:   grid.ConnectURL,
		}, updateChannels.GridUpdated(), gridConfig)
	case config.Victron:
		g, err = createModbusGrid(h, grid, gridConfig, updateChannels, victron.NewModbusConfiguration, victron.NewGrid)
		if err != nil {
			log.Fatalf("Unable to create Carlo Gavazzi grid: %s", err.Error())
			panic(err)
		}
	}
	if g != nil {
		h.system.SetGrid(g)
	}
}

func createModbusGrid(h *home,
	grid *config.Grid,
	gridConfig *energysource.GridConfig,
	updateChannels *internal.UpdateChannels,
	newModbusConfiguration func(string) *modbusProtocol.ClientConfiguration,
	newGrid func(name string, config *modbus.GridConfig, updateLoop *modbus.UpdateLoop) (*modbus.Grid, error),
) (*modbus.Grid, error) {
	h.addModbusUpdateLoop(grid.ConnectURL, updateChannels, newModbusConfiguration)
	mbConfig := &modbus.GridConfig{
		GridConfig: gridConfig,
	}
	if len(grid.Meters) > 0 {
		mbConfig.ModbusMeters = make([]*modbus.MeterConfig, len(grid.Meters))
		for ix, meter := range grid.Meters {
			mbConfig.ModbusMeters[ix] = &modbus.MeterConfig{
				ModbusUnitId: meter.ModbusUnitId,
				LineIndices:  meter.LineIndices,
			}

		}
	}
	g, err := newGrid(grid.Name, mbConfig, h.modbusUpdateLoops[grid.ConnectURL])
	if err != nil {
		return nil, err
	}
	return g, nil
}

func addPvs(h *home, pvs []*config.Pv, updateChannels *internal.UpdateChannels) {
	if pvs == nil || len(pvs) == 0 {
		return
	}
	pvConfig := energysource.NewPvConfig()
	for _, pv := range pvs {
		switch pv.Brand {
		case config.ABB:
			p, err := createModbusPv(h, pv, pvConfig, updateChannels, abb.NewModbusConfiguration, abb.NewPv)
			if err != nil {
				log.Fatalf("Unable to create ABB pv: %s", err.Error())
				panic(err)
			}
			h.system.AddPv(p)
		case config.CarloGavazzi:
			p, err := createModbusPv(h, pv, pvConfig, updateChannels, carlo_gavazzi.NewModbusConfiguration, carlo_gavazzi.NewPv)
			if err != nil {
				log.Fatalf("Unable to create Carlo Gavazzi pv: %s", err.Error())
				panic(err)
			}
			h.system.AddPv(p)
		case config.Victron:
			p, err := createModbusPv(h, pv, pvConfig, updateChannels, victron.NewModbusConfiguration, victron.NewPv)
			if err != nil {
				log.Fatalf("Unable to create Victron pv: %s", err.Error())
				panic(err)
			}
			h.system.AddPv(p)
		}
	}
}

func createModbusPv(h *home,
	pv *config.Pv,
	pvConfig *energysource.PvConfig,
	updateChannels *internal.UpdateChannels,
	newModbusConfiguration func(string) *modbusProtocol.ClientConfiguration,
	newPv func(name string, config *modbus.PvConfig, updateLoop *modbus.UpdateLoop) (*modbus.Pv, error),
) (*modbus.Pv, error) {
	h.addModbusUpdateLoop(pv.ConnectURL, updateChannels, newModbusConfiguration)
	mbConfig := &modbus.PvConfig{
		PvConfig: pvConfig,
	}
	if len(pv.Meters) > 0 {
		mbConfig.ModbusMeters = make([]*modbus.MeterConfig, len(pv.Meters))
		for ix, meter := range pv.Meters {
			mbConfig.ModbusMeters[ix] = &modbus.MeterConfig{
				ModbusUnitId: meter.ModbusUnitId,
				LineIndices:  meter.LineIndices,
			}
		}
	}
	p, err := newPv(pv.Name, mbConfig, h.modbusUpdateLoops[pv.ConnectURL])
	if err != nil {
		return nil, err
	}
	return p, nil
}

func startModbusServer(config *config.Configuration, home *home) []*modbusProtocol.ModbusServer {
	if config.ModbusServers == nil {
		return nil
	}
	var servers []*modbusProtocol.ModbusServer
	requestHandler := proxy.NewDispatchingRequestHandler()
	if home.system.Grid() != nil && config.Grid.ModbusMeterSimulator != nil {
		simulator := proxy.NewMeterSimulator(config.Grid.ModbusMeterSimulator.MeterType, config.Grid.ModbusMeterSimulator.ModbusUnitId, home.system.Grid())
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
			for _, p := range home.system.Pvs() {
				if p.Name() == pv.Name {
					log.Infof("Adding %s energy meter simulator for Pv %s at unit id %d", pv.ModbusMeterSimulator.MeterType, pv.Name, pv.ModbusMeterSimulator.ModbusUnitId)
					simulator := proxy.NewMeterSimulator(pv.ModbusMeterSimulator.MeterType, pv.ModbusMeterSimulator.ModbusUnitId, p)
					if simulator != nil {
						requestHandler.AddHandler(pv.ModbusMeterSimulator.ModbusUnitId, simulator)
					}
					break
				}
			}
		}
	}

	for _, modbusProxy := range config.ModbusServers {
		server, err := modbusProtocol.NewServer(&modbusProtocol.ServerConfiguration{
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
		err = server.Start()
		if err != nil {
			log.Errorf("Failed to start modbus proxy server: %s", err.Error())
			continue
		}
		servers = append(servers, server)
		log.Infof("Started modbus proxy on %s", modbusProxy.ServerUrl)
	}
	return servers
}

func (h *home) printStatusAsHtml(w http.ResponseWriter, r *http.Request) {
	if h.system.Grid() == nil {
		_, _ = io.WriteString(w, "Grid not found")
		return
	}
	g := h.system.Grid()
	_, _ = io.WriteString(w, fmt.Sprintf("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
		g.Phases(),
		g.TotalPower(), g.Power(0), g.Power(1), g.Power(2),
		g.TotalCurrent(), g.Current(0), g.Current(1), g.Current(2),
		g.Voltage(0), g.Voltage(1), g.Voltage(2)))
}

func (h *home) dataAsJson(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(map[string]any{
		"system": h.system.ToMap(),
	})
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	_, _ = w.Write(data)
	//g := *h.system.Grid()
	//_, _ = io.WriteString(w, fmt.Sprintf("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
	//	g.Phases(),
	//	g.TotalPower(), g.Power(0), g.Power(1), g.Power(2),
	//	g.TotalCurrent(), g.Current(0), g.Current(1), g.Current(2),
	//	g.Voltage(0), g.Voltage(1), g.Voltage(2)))
}

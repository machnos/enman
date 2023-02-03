package main

import (
	"encoding/json"
	"enman/internal"
	"enman/internal/balance"
	"enman/internal/config"
	"enman/internal/energysource"
	"enman/internal/energysource/modbus"
	"enman/internal/energysource/modbus/carlo_gavazzi"
	"enman/internal/energysource/modbus/dsmr"
	"enman/internal/energysource/modbus/victron"
	"enman/internal/log"
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

func main() {
	log.ActiveLevel = log.LvlInfo

	configFile := *flag.String("config-file", "config.json", "Full path to the configuration file")
	flag.Parse()

	configuration := config.LoadConfiguration(configFile)
	if configuration == nil {
		syscall.Exit(-1)
	}
	var repository persistency.Repository
	if configuration.Influx != nil {
		repository = persistency.NewInfluxRepository(configuration.Influx.ServerUrl, configuration.Influx.Token)
		defer repository.Close()
	} else {
		repository = persistency.NewNoopRepository()
	}

	h := &home{
		system:            &internal.System{},
		modbusUpdateLoops: make(map[string]*modbus.UpdateLoop),
	}
	updateChannels := internal.NewUpdateChannels()
	addGrid(h, configuration.Grid, updateChannels)
	addPvs(h, configuration.Pvs, updateChannels)
	defer h.Close()
	balance.StartUpdateLoop(updateChannels, repository)
	mux := http.NewServeMux()

	mux.HandleFunc("/", h.printStatusAsHtml)
	mux.HandleFunc("/api", h.dataAsJson)

	//http.ListenAndServe uses the default server structure.
	err := http.ListenAndServe(fmt.Sprintf(":%d", configuration.Http.Port), mux)
	if err != nil {
		log.Fatal(err.Error())
	}
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
	case config.CarloGavazzi:
		if h.modbusUpdateLoops[grid.ConnectURL] == nil {
			updateLoop, err := modbus.NewUpdateLoop(carlo_gavazzi.NewModbusConfiguration(grid.ConnectURL), updateChannels)
			if err != nil {
				log.Fatalf("Unable to setup modbus connection to %s: %s", grid.ConnectURL, err.Error())
				panic(err)
			}
			h.modbusUpdateLoops[grid.ConnectURL] = updateLoop
		}
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
		g, err = carlo_gavazzi.NewGrid(grid.Name, mbConfig, h.modbusUpdateLoops[grid.ConnectURL])
		if err != nil {
			log.Fatalf("Unable to create Carlo Gavazzi grid: %s", err.Error())
			panic(err)
		}
	case config.DSMR:
		g, err = dsmr.NewDsmrGrid(&dsmr.DsmrConfig{
			BaudRate: 115200,
			Device:   grid.ConnectURL,
		}, updateChannels.GridUpdated(), gridConfig)
	case config.Victron:
		if h.modbusUpdateLoops[grid.ConnectURL] == nil {
			updateLoop, err := modbus.NewUpdateLoop(victron.NewModbusConfiguration(grid.ConnectURL), updateChannels)
			if err != nil {
				log.Fatalf("Unable to setup modbus connection to %s: %s", grid.ConnectURL, err.Error())
				panic(err)
			}
			h.modbusUpdateLoops[grid.ConnectURL] = updateLoop
		}
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
		g, err = victron.NewGrid(grid.Name, mbConfig, h.modbusUpdateLoops[grid.ConnectURL])
		if err != nil {
			log.Fatalf("Unable to create Carlo Gavazzi grid: %s", err.Error())
			panic(err)
		}

	}
	if g != nil {
		h.system.SetGrid(g)
	}
}

func addPvs(h *home, pvs []*config.Pv, updateChannels *internal.UpdateChannels) {
	if pvs == nil || len(pvs) == 0 {
		return
	}
	pvConfig := energysource.NewPvConfig()
	for _, pv := range pvs {
		switch pv.Brand {
		case config.CarloGavazzi:
			if h.modbusUpdateLoops[pv.ConnectURL] == nil {
				updateLoop, err := modbus.NewUpdateLoop(carlo_gavazzi.NewModbusConfiguration(pv.ConnectURL), updateChannels)
				if err != nil {
					log.Fatalf("Unable to setup modbus connection to %s: %s", pv.ConnectURL, err.Error())
					panic(err)
				}
				h.modbusUpdateLoops[pv.ConnectURL] = updateLoop
			}
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
			p, err := carlo_gavazzi.NewPv(pv.Name, mbConfig, h.modbusUpdateLoops[pv.ConnectURL])
			if err != nil {
				log.Fatalf("Unable to create Carlo Gavazzi pv: %s", err.Error())
				panic(err)
			}
			h.system.AddPv(p)
		case config.Victron:
			if h.modbusUpdateLoops[pv.ConnectURL] == nil {
				updateLoop, err := modbus.NewUpdateLoop(victron.NewModbusConfiguration(pv.ConnectURL), updateChannels)
				if err != nil {
					log.Fatalf("Unable to setup modbus connection to %s: %s", pv.ConnectURL, err.Error())
					panic(err)
				}
				h.modbusUpdateLoops[pv.ConnectURL] = updateLoop
			}
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
			p, err := victron.NewPv(pv.Name, mbConfig, h.modbusUpdateLoops[pv.ConnectURL])
			if err != nil {
				log.Fatalf("Unable to create Victron pv: %s", err.Error())
				panic(err)
			}
			h.system.AddPv(p)
		}
	}
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

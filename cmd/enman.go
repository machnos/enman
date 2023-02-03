package main

import (
	"encoding/json"
	"enman/internal/balance"
	"enman/internal/config"
	ies "enman/internal/energysource"
	"enman/internal/log"
	"enman/internal/persistency"
	"enman/pkg/energysource"
	"fmt"
	"io"
	"net/http"
	"syscall"
)

type home struct {
	system *energysource.System
}

func main() {
	log.ActiveLevel = log.LvlInfo

	configuration := config.LoadConfiguration()
	if configuration == nil {
		syscall.Exit(-1)
	}
	var repository persistency.Repository
	if configuration.Influx != nil {
		//repository = persistency.NewInfluxRepository("http://127.0.0.1:8086", "JaLvEBCyFj9n_rXjPmg5eLLBCY87hfz0e2lldSh9egyeIBTVTR2i270MpMYcCyEScP29G9rJmHPrXHarQOiMPA==")
		repository = persistency.NewInfluxRepository(configuration.Influx.ServerUrl, configuration.Influx.Token)
		defer repository.Close()
	} else {
		repository = persistency.NewNoopRepository()
	}

	//repository := persistency.NewInfluxRepository("http://127.0.0.1:8086", "JaLvEBCyFj9n_rXjPmg5eLLBCY87hfz0e2lldSh9egyeIBTVTR2i270MpMYcCyEScP29G9rJmHPrXHarQOiMPA==")

	h := &home{}
	updateChannels := energysource.NewUpdateChannels()
	addGrid(h, configuration.Grid, updateChannels)
	addPvs(h, configuration.Pvs, updateChannels)

	//config := &ies.VictronConfig{
	//	ModbusUrl: "tcp://einstein.energy.cleme:502",
	//	ModbusGridConfig: &ies.ModbusGridConfig{
	//		Grid: gridConfig,
	//		ModbusMeters: []*ies.ModbusMeterConfig{
	//			{
	//				ModbusUnitId: 31,
	//				LineIndices:  []uint8{0, 1, 2},
	//			},
	//		},
	//	},
	//}
	//system, err := ies.NewVictronSystem(config, updateChannels)

	//config := &ies.CarloGavazziConfig{
	//	ModbusUrl: "rtu:///dev/ttyUSB0",
	//	ModbusGridConfig: &ies.ModbusGridConfig{
	//		Grid: gridConfig,
	//		ModbusMeters: []*ies.ModbusMeterConfig{
	//			{
	//				ModbusUnitId: 2,
	//				LineIndices:  []uint8{0, 1, 2},
	//			},
	//		},
	//	},
	//	ModbusPvConfigs: []*ies.ModbusPvConfig{
	//		{
	//			Pv: energysource.NewPvConfig("Enphase"),
	//			ModbusMeters: []*ies.ModbusMeterConfig{
	//				{
	//					ModbusUnitId: 3,
	//					LineIndices:  []uint8{0},
	//				},
	//				{
	//					ModbusUnitId: 4,
	//					LineIndices:  []uint8{1},
	//				},
	//			},
	//		},
	//	},
	//}
	//system, err := ies.NewCarloGavazziSystem(config, updateChannels)

	balance.StartUpdateLoop(updateChannels, repository)
	mux := http.NewServeMux()

	mux.HandleFunc("/", h.printStatusAsHtml)
	mux.HandleFunc("/api", h.dataAsJson)

	//http.ListenAndServe uses the default server structure.
	err := http.ListenAndServe(":8081", mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func addGrid(h *home, grid *config.Grid, updateChannels *energysource.UpdateChannels) {
	gridConfig, err := energysource.NewGridConfig(
		grid.Name,
		float32(grid.Voltage),
		float32(grid.MaxCurrent),
		grid.Phases,
	)
	if err != nil {
		log.Fatalf("Unable to create grid configuration: %s", err.Error())
		panic(err)
	}
	var system *energysource.System
	switch grid.Brand {
	case config.CarloGavazzi:
		cfg := &ies.CarloGavazziConfig{
			ModbusUrl: grid.ConnectURL,
			ModbusGridConfig: &ies.ModbusGridConfig{
				Grid: gridConfig,
			},
		}
		if len(grid.Meters) > 0 {
			cfg.ModbusGridConfig.ModbusMeters = make([]*ies.ModbusMeterConfig, len(grid.Meters))
			for ix, meter := range grid.Meters {
				cfg.ModbusGridConfig.ModbusMeters[ix] = &ies.ModbusMeterConfig{
					ModbusUnitId: meter.ModbusUnitId,
					LineIndices:  meter.LineIndices,
				}
			}
		}
		system, err = ies.NewCarloGavazziSystem(cfg, updateChannels)
	case config.DSMR:
		system, err = ies.NewDsmrSystem(&ies.DsmrConfig{
			BaudRate: 115200,
			Device:   grid.ConnectURL,
		}, updateChannels, gridConfig)
	case config.Victron:
		cfg := &ies.VictronConfig{
			ModbusUrl: grid.ConnectURL,
			ModbusGridConfig: &ies.ModbusGridConfig{
				Grid: gridConfig,
			},
		}
		if len(grid.Meters) > 0 {
			cfg.ModbusGridConfig.ModbusMeters = make([]*ies.ModbusMeterConfig, len(grid.Meters))
			for ix, meter := range grid.Meters {
				cfg.ModbusGridConfig.ModbusMeters[ix] = &ies.ModbusMeterConfig{
					ModbusUnitId: meter.ModbusUnitId,
					LineIndices:  meter.LineIndices,
				}
			}
		}
		system, err = ies.NewVictronSystem(cfg, updateChannels)
	}
	if err != nil {
		log.Fatalf("Unable to load grid meters: %s", err.Error())
		panic(err)
	}
	if h.system == nil {
		h.system = system
	} else {
		h.system.Merge(system)
	}
}

func addPvs(h *home, pvs []*config.Pv, updateChannels *energysource.UpdateChannels) {
	if pvs == nil || len(pvs) == 0 {
		return
	}
	var system *energysource.System
	var err error
	for ixPv, pv := range pvs {
		switch pv.Brand {
		case config.CarloGavazzi:
			cfg := &ies.CarloGavazziConfig{
				ModbusUrl:       pv.ConnectURL,
				ModbusPvConfigs: make([]*ies.ModbusPvConfig, len(pvs)),
			}
			if len(pv.Meters) > 0 {
				cfg.ModbusPvConfigs[ixPv].ModbusMeters = make([]*ies.ModbusMeterConfig, len(pv.Meters))
				for ixM, meter := range pv.Meters {
					cfg.ModbusPvConfigs[ixPv].ModbusMeters[ixM] = &ies.ModbusMeterConfig{
						ModbusUnitId: meter.ModbusUnitId,
						LineIndices:  meter.LineIndices,
					}
				}
			}
			system, err = ies.NewCarloGavazziSystem(cfg, updateChannels)
		case config.Victron:
			cfg := &ies.VictronConfig{
				ModbusUrl:       pv.ConnectURL,
				ModbusPvConfigs: make([]*ies.ModbusPvConfig, len(pvs)),
			}
			if len(pv.Meters) > 0 {
				cfg.ModbusPvConfigs[ixPv].ModbusMeters = make([]*ies.ModbusMeterConfig, len(pv.Meters))
				for ixM, meter := range pv.Meters {
					cfg.ModbusPvConfigs[ixPv].ModbusMeters[ixM] = &ies.ModbusMeterConfig{
						ModbusUnitId: meter.ModbusUnitId,
						LineIndices:  meter.LineIndices,
					}
				}
			}
			system, err = ies.NewVictronSystem(cfg, updateChannels)
		}
		if err != nil {
			log.Fatalf("Unable to load grid meters: %s", err.Error())
			panic(err)
		}
		if h.system == nil {
			h.system = system
		} else {
			h.system.Merge(system)
		}
	}
}

func (h home) printStatusAsHtml(w http.ResponseWriter, r *http.Request) {
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

func (h home) dataAsJson(w http.ResponseWriter, r *http.Request) {
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

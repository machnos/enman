package main

import (
	"encoding/json"
	ies "enman/internal/energysource"
	"enman/internal/log"
	"enman/pkg/energysource"
	"fmt"
	"io"
	"net/http"
	"time"
)

type home struct {
	system *energysource.System
}

func main() {
	log.ActiveLevel = log.LvlInfo

	gridConfig, err := energysource.NewGridConfig(230, 25, 3)
	if err != nil {
		panic(err)
	}

	//config := &ies.VictronConfig{
	//	ModbusUrl: "tcp://einstein.energy.cleme:502",
	//	ModbusGridConfig: &ies.ModbusGridConfig{
	//		Grid: gridConfig,
	//		ModbusMeters: []*ies.ModbusMeterConfig{
	//			{
	//				ModbusUnitId: 31,
	//				LineIndexes:  []uint8{0, 1, 2},
	//			},
	//		},
	//	},
	//}
	//system, err := ies.NewVictronSystem(config)
	//
	config := &ies.CarloGavazziConfig{
		ModbusUrl: "rtu:///dev/ttyUSB0",
		ModbusGridConfig: &ies.ModbusGridConfig{
			Grid: gridConfig,
			ModbusMeters: []*ies.ModbusMeterConfig{
				{
					ModbusUnitId: 2,
					LineIndexes:  []uint8{0, 1, 2},
				},
			},
		},
		ModbusPvConfigs: []*ies.ModbusPvConfig{
			{
				Pv: &energysource.PvConfig{},
				ModbusMeters: []*ies.ModbusMeterConfig{
					{
						ModbusUnitId: 3,
						LineIndexes:  []uint8{0},
					},
					{
						ModbusUnitId: 4,
						LineIndexes:  []uint8{1},
					},
				},
			},
		},
	}
	system, err := ies.NewCarloGavazziSystem(config)

	//system, err := ies.NewDsmrSystem(&ies.DsmrConfig{
	//	BaudRate: 115200,
	//	Device:   "/dev/ttyUSB0",
	//}, gridConfig)

	system.StartBalanceLoop()

	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/", home{system}.printStatusAsHtml)
	mux.HandleFunc("/api", home{system}.dataAsJson)

	//http.ListenAndServe uses the default server structure.
	err = http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func printUsage(system *energysource.System) {
	ticker := time.NewTicker(time.Millisecond * 1000)
	for {
		select {
		case <-ticker.C:
			if system.Grid() != nil {
				grid := system.Grid()
				log.Infof("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
					grid.Phases(),
					grid.TotalPower(), grid.Power(0), grid.Power(1), grid.Power(2),
					grid.TotalCurrent(), grid.Current(0), grid.Current(1), grid.Current(2),
					grid.Voltage(0), grid.Voltage(1), grid.Voltage(2))
			}

			if system.Pvs() != nil {
				pvs := system.Pvs()
				for ix := 0; ix < len(pvs); ix++ {
					pv := pvs[0]
					log.Infof("PV phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
						pv.Phases(),
						pv.TotalPower(), pv.Power(0), pv.Power(1), pv.Power(2),
						pv.TotalCurrent(), pv.Current(0), pv.Current(1), pv.Current(2),
						pv.Voltage(0), pv.Voltage(1), pv.Voltage(2))
				}
			}
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
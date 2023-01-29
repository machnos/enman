package energysource

import (
	"fmt"
	"math"
	"strings"
)

type System struct {
	grid        Grid
	pvs         []Pv
	loadUpdated chan bool
}

func (s *System) Grid() Grid {
	return s.grid
}

func (s *System) Pvs() []Pv {
	return s.pvs
}

func (s *System) LoadUpdated() chan bool {
	return s.loadUpdated
}

func (s *System) StartBalanceLoop() {
	for {
		select {
		case <-s.LoadUpdated():
			//if s.Grid() != nil {
			//	grid := s.Grid()
			//	log.Infof("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
			//		grid.Phases(),
			//		grid.TotalPower(), grid.Power(0), grid.Power(1), grid.Power(2),
			//		grid.TotalCurrent(), grid.Current(0), grid.Current(1), grid.Current(2),
			//		grid.Voltage(0), grid.Voltage(1), grid.Voltage(2))
			//	if s.Pvs() != nil {
			//		pvs := s.Pvs()
			//		for ix := 0; ix < len(pvs); ix++ {
			//			pv := pvs[0]
			//			log.Infof("PV phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV)",
			//				pv.Phases(),
			//				pv.TotalPower(), pv.Power(0), pv.Power(1), pv.Power(2),
			//				pv.TotalCurrent(), pv.Current(0), pv.Current(1), pv.Current(2),
			//				pv.Voltage(0), pv.Voltage(1), pv.Voltage(2))
			//		}
			//	}
			//}
			if s.Grid() != nil {
				grid := s.Grid()
				phases := grid.Phases()
				var b strings.Builder
				b.WriteString("Battery should ")
				for ix := uint8(0); ix < phases; ix++ {
					power := grid.Power(ix)
					if ix > 0 && ix <= (phases-2) {
						b.WriteString(", ")
					} else if ix > 0 && ix == (phases-1) {
						b.WriteString(" and ")
					}
					if power < 0 {
						_, _ = fmt.Fprintf(&b, "consumer %4.2fW on L%d", math.Abs(float64(power)), ix+1)
					} else {
						_, _ = fmt.Fprintf(&b, "provide %4.2fW on L%d", math.Abs(float64(power)), ix+1)
					}
				}
				if grid.TotalPower() < 0 {
					_, _ = fmt.Fprintf(&b, ". If phase compensation is enabled the battery should consume %4.2fW", math.Abs(float64(grid.TotalPower())))
				} else {
					_, _ = fmt.Fprintf(&b, ". If phase compensation is enabled the battery should provide %4.2fW", grid.TotalPower())
				}
				println(b.String())
			}
		}
	}
}

func (s *System) ToMap() map[string]any {
	data := map[string]any{}
	if s.Grid() != nil {
		data["grid"] = s.Grid().ToMap()
	}
	if s.Pvs() != nil {
		var pvData []map[string]any
		for ix := 0; ix < len(s.Pvs()); ix++ {
			pvData = append(pvData, s.Pvs()[ix].ToMap())
		}
		data["pvs"] = pvData
	}
	return data
}

func NewSystem(grid Grid, pvs []Pv) *System {
	system := &System{
		grid:        grid,
		pvs:         pvs,
		loadUpdated: make(chan bool),
	}
	return system
}

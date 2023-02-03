package balance

import (
	"enman/internal"
	"enman/internal/persistency"
)

func StartUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	go startGridUpdateLoop(updateChannels, repository)
	go startPvUpdateLoop(updateChannels, repository)
}

func startGridUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	for {
		grid := <-updateChannels.GridUpdated()
		repository.StoreGridValues(grid)

		//log.Infof("Phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV), Total consumed: %4.2fkWh, Total provided: %4.2fkWh",
		//	grid.Phases(),
		//	grid.TotalPower(), grid.Power(0), grid.Power(1), grid.Power(2),
		//	grid.TotalCurrent(), grid.Current(0), grid.Current(1), grid.Current(2),
		//	grid.Voltage(0), grid.Voltage(1), grid.Voltage(2),
		//	grid.TotalEnergyConsumed(), grid.TotalEnergyProvided(),
		//)
		//phases := grid.Phases()
		//var b strings.Builder
		//b.WriteString("Battery should ")
		//for ix := uint8(0); ix < phases; ix++ {
		//	power := grid.Power(ix)
		//	if ix > 0 && ix <= (phases-2) {
		//		b.WriteString(", ")
		//	} else if ix > 0 && ix == (phases-1) {
		//		b.WriteString(" and ")
		//	}
		//	if power < 0 {
		//		_, _ = fmt.Fprintf(&b, "consumer %4.2fW on L%d", math.Abs(float64(power)), ix+1)
		//	} else {
		//		_, _ = fmt.Fprintf(&b, "provide %4.2fW on L%d", math.Abs(float64(power)), ix+1)
		//	}
		//}
		//if grid.TotalPower() < 0 {
		//	_, _ = fmt.Fprintf(&b, ". If phase compensation is enabled the battery should consume %4.2fW", math.Abs(float64(grid.TotalPower())))
		//} else {
		//	_, _ = fmt.Fprintf(&b, ". If phase compensation is enabled the battery should provide %4.2fW", grid.TotalPower())
		//}
		//println(b.String())
	}
}

func startPvUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	for {
		pv := <-updateChannels.PvUpdated()
		repository.StorePvValues(pv)
		//log.Infof("PV phases: %d, Power %4.2fW (L1: %4.2fW, L2: %4.2fW, L3: %4.2fW), Current %4.2fA (L1: %4.2fA, L2: %4.2fA, L3: %4.2fA), Voltage (L1: %4.2fV, L2: %4.2fV, L3: %4.2fV), Total consumed: %4.2fkWh, Total provided: %4.2fkWh",
		//	pv.Phases(),
		//	pv.TotalPower(), pv.Power(0), pv.Power(1), pv.Power(2),
		//	pv.TotalCurrent(), pv.Current(0), pv.Current(1), pv.Current(2),
		//	pv.Voltage(0), pv.Voltage(1), pv.Voltage(2),
		//	pv.TotalEnergyConsumed(), pv.TotalEnergyProvided(),
		//)
	}
}

package domain

import (
	"context"
	"time"
)

type Pv struct {
	name             string
	electricityState *ElectricityState
	electricityUsage *ElectricityUsage
	meters           []EnergyMeter
	updateTicker     *time.Ticker
}

func (pv *Pv) Name() string {
	return pv.name
}

func (pv *Pv) Role() EnergySourceRole {
	return RolePv
}

func (pv *Pv) ElectricityState() *ElectricityState {
	return pv.electricityState
}

func (pv *Pv) ElectricityUsage() *ElectricityUsage {
	return pv.electricityUsage
}

func (pv *Pv) StartMeasuring(context context.Context) {
	if pv.updateTicker != nil {
		// Meter already started
		return
	}
	// Look for the highest update interval
	interval := time.Millisecond
	for _, meter := range pv.meters {
		if meter.UpdateInterval() > interval {
			interval = meter.UpdateInterval()
		}
	}
	pv.updateTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-context.Done():
				for _, meter := range pv.meters {
					meter.Shutdown()
				}
				return
			case _ = <-pv.updateTicker.C:
				es := NewElectricityState()
				eu := NewElectricityUsage()
				for _, meter := range pv.meters {
					meter.UpdateValues(es, eu, nil, nil)
				}
				pv.electricityState = es
				pv.electricityUsage = eu
			}
		}
	}()
}

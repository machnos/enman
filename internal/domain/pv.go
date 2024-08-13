package domain

import (
	"context"
	"enman/internal/log"
	"sort"
	"time"
)

type PvController interface {
	DisablePv() error
	EnablePv() error
}

type Pv struct {
	name             string
	electricityState *ElectricityState
	electricityUsage *ElectricityUsage
	meters           []EnergyMeter
	controller       PvController
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
	// Look for the highest update interval, phases and read line indices.
	interval := time.Millisecond
	meterPhases := uint8(0)
	readLineIndices := make([]uint8, 0)
	for _, meter := range pv.meters {
		if meter.UpdateInterval() > interval {
			interval = meter.UpdateInterval()
		}
		electricityMeter, ok := meter.(ElectricityMeter)
		if ok {
			meterPhases += electricityMeter.Phases()
			readLineIndices = append(readLineIndices, electricityMeter.LineIndices()...)
		}
	}
	sort.Slice(readLineIndices, func(i, j int) bool {
		return readLineIndices[i] < readLineIndices[j]
	})
	pv.updateTicker = time.NewTicker(interval)

	go func() {
		var usageLastRead time.Time
		for {
			select {
			case <-context.Done():
				for _, meter := range pv.meters {
					meter.Shutdown()
				}
				return
			case _ = <-pv.updateTicker.C:
				es := NewElectricityState()
				var eu *ElectricityUsage = nil
				if usageLastRead.IsZero() || (time.Now().Sub(usageLastRead) > electricityMeterUsageUpdateInterval) {
					usageLastRead = time.Now()
					eu = NewElectricityUsage()
				}
				for _, meter := range pv.meters {
					err := meter.UpdateValues(es, eu, nil, nil, nil)
					if err != nil {
						log.Debugf("Failed to read pv values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
					}
				}
				pv.electricityState.SetValues(es)
				if eu != nil && !eu.IsZero() {
					pv.electricityUsage.SetValues(eu)
				}
				if !es.IsZero() || (eu != nil && !eu.IsZero()) {
					electricityMeterValues := NewElectricityMeterValues().
						SetName(pv.Name()).
						SetRole(pv.Role()).
						SetElectricityState(es).
						SetElectricityUsage(eu).
						SetMeterPhases(meterPhases).
						SetReadLineIndices(readLineIndices)
					ElectricityMeterReadings.Trigger(electricityMeterValues)
				}
			}
		}
	}()
}

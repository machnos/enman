package domain

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/events"
	"enman/internal/log"
	"sort"
	"time"
)

type PvController interface {
	DisablePv() error
	EnablePv() error
}

type Pv struct {
	name         string
	state        *electricity.State
	usage        *electricity.Usage
	meters       []EnergyMeter
	controller   PvController
	updateTicker *time.Ticker
}

func (pv *Pv) Name() string {
	return pv.name
}

func (pv *Pv) Role() constants.EnergySourceRole {
	return constants.EnergySourceRolePv
}

func (pv *Pv) State() *electricity.State {
	return pv.state
}

func (pv *Pv) Usage() *electricity.Usage {
	return pv.usage
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
		electricityMeter, ok := meter.(electricity.Meter)
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
	loadLoop:
		for {
			select {
			case <-context.Done():
				for _, meter := range pv.meters {
					meter.Shutdown()
				}
				return
			case _ = <-pv.updateTicker.C:
				es := electricity.NewState()
				var eu *electricity.Usage = nil
				if usageLastRead.IsZero() || (time.Now().Sub(usageLastRead) > electricity.MeterUsageUpdateInterval) {
					usageLastRead = time.Now()
					eu = electricity.NewUsage()
				}
				for _, meter := range pv.meters {
					err := meter.UpdateValues(es, eu, nil, nil, nil)
					if err != nil {
						log.Debugf("Failed to read pv values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
						continue loadLoop
					}
				}
				pv.state.SetValues(es)
				if eu != nil && !eu.IsZero() {
					pv.usage.SetValues(eu)
				}
				if !es.IsZero() || (eu != nil && !eu.IsZero()) {
					electricityMeterValues := events.NewElectricityMeterValues().
						SetName(pv.Name()).
						SetRole(pv.Role()).
						SetState(es).
						SetUsage(eu).
						SetMeterPhases(meterPhases).
						SetReadLineIndices(readLineIndices)
					events.ElectricityMeterReadings.Trigger(electricityMeterValues)
				}
			}
		}
	}()
}

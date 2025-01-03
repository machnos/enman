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

type AcLoad struct {
	name               string
	role               constants.EnergySourceRole
	percentageFromGrid uint8
	state              *electricity.State
	usage              *electricity.Usage
	meters             []EnergyMeter
	updateTicker       *time.Ticker
}

func (acl *AcLoad) Name() string {
	return acl.name
}

func (acl *AcLoad) Role() constants.EnergySourceRole {
	return acl.role
}

func (acl *AcLoad) PercentageFromGrid() uint8 {
	return acl.percentageFromGrid
}

func (acl *AcLoad) State() *electricity.State {
	return acl.state
}

func (acl *AcLoad) Usage() *electricity.Usage {
	return acl.usage
}

func (acl *AcLoad) StartMeasuring(context context.Context) {
	if acl.updateTicker != nil {
		// Meter already started
		return
	}
	// Look for the highest update interval, phases and read line indices.
	interval := time.Millisecond
	meterPhases := uint8(0)
	readLineIndices := make([]uint8, 0)
	for _, meter := range acl.meters {
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
	acl.updateTicker = time.NewTicker(interval)

	go func() {
		var usageLastRead time.Time
	loadLoop:
		for {
			select {
			case <-context.Done():
				for _, meter := range acl.meters {
					meter.Shutdown()
				}
				return
			case _ = <-acl.updateTicker.C:
				es := electricity.NewState()
				var eu *electricity.Usage = nil
				if usageLastRead.IsZero() || (time.Now().Sub(usageLastRead) > electricity.MeterUsageUpdateInterval) {
					usageLastRead = time.Now()
					eu = electricity.NewUsage()
				}
				for _, meter := range acl.meters {
					err := meter.UpdateValues(es, eu, nil, nil, nil)
					if err != nil {
						log.Debugf("Failed to read ac load values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
						continue loadLoop
					}
				}
				acl.state.SetValues(es)
				if eu != nil && !eu.IsZero() {
					acl.usage.SetValues(eu)
				}
				if !es.IsZero() || (eu != nil && !eu.IsZero()) {
					electricityMeterValues := events.NewElectricityMeterValues().
						SetName(acl.Name()).
						SetRole(acl.Role()).
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

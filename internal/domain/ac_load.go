package domain

import (
	"context"
	"sort"
	"time"
)

type AcLoad struct {
	name             string
	role             EnergySourceRole
	electricityState *ElectricityState
	electricityUsage *ElectricityUsage
	meters           []EnergyMeter
	updateTicker     *time.Ticker
}

func (acl *AcLoad) Name() string {
	return acl.name
}

func (acl *AcLoad) Role() EnergySourceRole {
	return acl.role
}

func (acl *AcLoad) ElectricityState() *ElectricityState {
	return acl.electricityState
}

func (acl *AcLoad) ElectricityUsage() *ElectricityUsage {
	return acl.electricityUsage
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
		electricityMeter, ok := meter.(ElectricityMeter)
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
		for {
			select {
			case <-context.Done():
				for _, meter := range acl.meters {
					meter.Shutdown()
				}
				return
			case _ = <-acl.updateTicker.C:
				es := NewElectricityState()
				eu := NewElectricityUsage()
				for _, meter := range acl.meters {
					meter.UpdateValues(es, eu, nil, nil, nil)
				}
				acl.electricityState = es
				acl.electricityUsage = eu
				if !es.IsZero() || !eu.IsZero() {
					electricityMeterValues := NewElectricityMeterValues().
						SetName(acl.Name()).
						SetRole(acl.Role()).
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

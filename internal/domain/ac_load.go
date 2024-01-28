package domain

import (
	"context"
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
	// Look for the highest update interval
	interval := time.Millisecond
	for _, meter := range acl.meters {
		if meter.UpdateInterval() > interval {
			interval = meter.UpdateInterval()
		}
	}
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
					meter.UpdateValues(es, eu, nil, nil)
				}
				acl.electricityState = es
				acl.electricityUsage = eu
			}
		}
	}()
}

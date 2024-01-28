package domain

import (
	"context"
	"time"
)

type Grid struct {
	name               string
	voltage            uint16
	maxCurrentPerPhase float32
	phases             uint8
	electricityState   *ElectricityState
	electricityUsage   *ElectricityUsage
	gasUsage           *GasUsage
	waterUsage         *WaterUsage
	meters             []EnergyMeter
	updateTicker       *time.Ticker
}

func (g *Grid) Name() string {
	return g.name
}

func (g *Grid) Role() EnergySourceRole {
	return RoleGrid
}

func (g *Grid) Voltage() uint16 {
	return g.voltage
}

func (g *Grid) MaxCurrentPerPhase() float32 {
	return g.maxCurrentPerPhase
}

func (g *Grid) Phases() uint8 {
	return g.phases
}

func (g *Grid) ElectricityState() *ElectricityState {
	return g.electricityState
}
func (g *Grid) ElectricityUsage() *ElectricityUsage {
	return g.electricityUsage
}

func (g *Grid) StartMeasuring(context context.Context) {
	if g.updateTicker != nil {
		// Meter already started
		return
	}
	// Look for the highest update interval
	interval := time.Millisecond
	for _, meter := range g.meters {
		if meter.UpdateInterval() > interval {
			interval = meter.UpdateInterval()
		}
	}
	g.updateTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-context.Done():
				for _, meter := range g.meters {
					meter.Shutdown()
				}
				return
			case _ = <-g.updateTicker.C:
				es := NewElectricityState()
				eu := NewElectricityUsage()
				gu := NewGasUsage()
				wu := NewWaterUsage()
				for _, meter := range g.meters {
					meter.UpdateValues(es, eu, gu, wu)
				}
				g.electricityState = es
				g.electricityUsage = eu
				g.gasUsage = gu
				g.waterUsage = wu
				// TODO fire value changed events
			}
		}
	}()
}

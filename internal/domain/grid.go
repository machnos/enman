package domain

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/events"
	"enman/internal/domain/gas"
	"enman/internal/domain/water"
	"enman/internal/log"
	"sort"
	"time"
)

type GridController interface {
	SetTargetConsumption(targetConsumption int) error
	GridConnected() (bool, error)
}

type Grid struct {
	name               string
	voltage            uint16
	maxCurrentPerPhase float32
	phases             uint8
	targetConsumption  int
	meters             []EnergyMeter
	controller         GridController
	electricityState   *electricity.State
	electricityUsage   *electricity.Usage
	gasUsage           *gas.Usage
	waterUsage         *water.Usage
	updateTicker       *time.Ticker
}

func (g *Grid) Name() string {
	return g.name
}

func (g *Grid) Role() constants.EnergySourceRole {
	return constants.EnergySourceRoleGrid
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

func (g *Grid) TargetConsumption() int {
	return g.targetConsumption
}

func (g *Grid) ElectricityState() *electricity.State {
	return g.electricityState
}
func (g *Grid) ElectricityUsage() *electricity.Usage {
	return g.electricityUsage
}
func (g *Grid) GasUsage() *gas.Usage {
	return g.gasUsage
}
func (g *Grid) WaterUsage() *water.Usage {
	return g.waterUsage
}

func (g *Grid) StartMeasuring(context context.Context) {
	if g.updateTicker != nil {
		// Meter already started
		return
	}

	// Look for the highest update interval, phases and read line indices.
	interval := time.Millisecond
	meterPhases := uint8(0)
	readLineIndices := make([]uint8, 0)
	for _, meter := range g.meters {
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
	g.updateTicker = time.NewTicker(interval)

	go func() {
		var usageLastRead time.Time
	loadLoop:
		for {
			select {
			case <-context.Done():
				for _, meter := range g.meters {
					meter.Shutdown()
				}
				return
			case _ = <-g.updateTicker.C:
				es := electricity.NewState()
				var eu *electricity.Usage = nil
				var gu *gas.Usage = nil
				var wu *water.Usage = nil
				if usageLastRead.IsZero() || (time.Now().Sub(usageLastRead) > electricity.MeterUsageUpdateInterval) {
					usageLastRead = time.Now()
					eu = electricity.NewUsage()
					gu = gas.NewUsage()
					wu = water.NewUsage()
				}
				for _, meter := range g.meters {
					err := meter.UpdateValues(es, eu, gu, wu, nil)
					if err != nil {
						log.Debugf("Failed to read grid values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
						continue loadLoop
					}
				}
				g.electricityState.SetValues(es)
				if eu != nil && !eu.IsZero() {
					g.electricityUsage.SetValues(eu)
				}
				if !es.IsZero() || (eu != nil && !eu.IsZero()) {
					electricityMeterValues := events.NewElectricityMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetState(es).
						SetUsage(eu).
						SetMeterPhases(meterPhases).
						SetReadLineIndices(readLineIndices)
					events.ElectricityMeterReadings.Trigger(electricityMeterValues)
				}
				if gu != nil && !gu.IsZero() {
					g.gasUsage.SetValues(gu)
					gasMeterValues := events.NewGasMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetUsage(gu)
					events.GasMeterReadings.Trigger(gasMeterValues)
				}
				if wu != nil && !wu.IsZero() {
					g.waterUsage.SetValues(wu)
					waterMeterValues := events.NewWaterMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetUsage(wu)
					events.WaterMeterReadings.Trigger(waterMeterValues)
				}
			}
		}
	}()
}

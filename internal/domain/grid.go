package domain

import (
	"context"
	"enman/internal/log"
	"sort"
	"time"
)

type GridController interface {
	SetTargetConsumption(targetConsumption int) error
}

type Grid struct {
	name               string
	voltage            uint16
	maxCurrentPerPhase float32
	phases             uint8
	targetConsumption  int
	meters             []EnergyMeter
	controller         GridController
	electricityState   *ElectricityState
	electricityUsage   *ElectricityUsage
	gasUsage           *GasUsage
	waterUsage         *WaterUsage
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

func (g *Grid) TargetConsumption() int {
	return g.targetConsumption
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

	// Look for the highest update interval, phases and read line indices.
	interval := time.Millisecond
	meterPhases := uint8(0)
	readLineIndices := make([]uint8, 0)
	for _, meter := range g.meters {
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
	g.updateTicker = time.NewTicker(interval)

	go func() {
		var usageLastRead time.Time
	outer:
		for {
			select {
			case <-context.Done():
				for _, meter := range g.meters {
					meter.Shutdown()
				}
				return
			case _ = <-g.updateTicker.C:
				es := NewElectricityState()
				var eu *ElectricityUsage = nil
				var gu *GasUsage = nil
				var wu *WaterUsage = nil
				if usageLastRead.IsZero() || (time.Now().Sub(usageLastRead) > electricityMeterUsageUpdateInterval) {
					usageLastRead = time.Now()
					eu = NewElectricityUsage()
					gu = NewGasUsage()
					wu = NewWaterUsage()
				}
				for _, meter := range g.meters {
					err := meter.UpdateValues(es, eu, gu, wu, nil)
					if err != nil {
						if log.DebugEnabled() {
							log.Debugf("Failed to read grid values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
						}
						continue outer
					}
				}
				g.electricityState.SetValues(es)
				if eu != nil && !eu.IsZero() {
					g.electricityUsage.SetValues(eu)
				}
				if !es.IsZero() || (eu != nil && !eu.IsZero()) {
					electricityMeterValues := NewElectricityMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetElectricityState(es).
						SetElectricityUsage(eu).
						SetMeterPhases(meterPhases).
						SetReadLineIndices(readLineIndices)
					ElectricityMeterReadings.Trigger(electricityMeterValues)
				}
				if gu != nil && !gu.IsZero() {
					g.gasUsage.SetValues(gu)
					gasMeterValues := NewGasMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetGasUsage(gu)
					GasMeterReadings.Trigger(gasMeterValues)
				}
				if wu != nil && !wu.IsZero() {
					g.waterUsage.SetValues(wu)
					waterMeterValues := NewWaterMeterValues().
						SetName(g.Name()).
						SetRole(g.Role()).
						SetWaterUsage(wu)
					WaterMeterReadings.Trigger(waterMeterValues)
				}
			}
		}
	}()
}

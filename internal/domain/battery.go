package domain

import (
	"context"
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/events"
	"enman/internal/log"
	"sort"
	"time"
)

type Battery struct {
	name                   string
	capacity               uint16
	chargingCoefficient    float32
	dischargingCoefficient float32
	voltage                float32
	state                  *battery.State
	meters                 []EnergyMeter
	updateTicker           *time.Ticker
}

func (b *Battery) Name() string {
	return b.name
}

func (b *Battery) Role() constants.EnergySourceRole {
	return constants.EnergySourceRoleBattery
}

func (b *Battery) Capacity() uint16 {
	return b.capacity
}

func (b *Battery) ChargingCoefficient() float32 {
	return b.chargingCoefficient
}

func (b *Battery) DischargingCoefficient() float32 {
	return b.dischargingCoefficient
}

func (b *Battery) Voltage() float32 {
	return b.voltage
}

func (b *Battery) State() *battery.State {
	return b.state
}

func (b *Battery) MaxChargePower() float32 {
	if b.Capacity() == 0 || b.Voltage() == 0 || b.ChargingCoefficient() == 0 {
		return 0
	}
	return float32(b.Capacity()) * b.ChargingCoefficient() * b.Voltage()
}

func (b *Battery) MaxDischargePower() float32 {
	if b.Capacity() == 0 || b.Voltage() == 0 || b.DischargingCoefficient() == 0 {
		return 0
	}
	return float32(b.Capacity()) * b.DischargingCoefficient() * b.Voltage()
}

func (b *Battery) AvailableCapacity() float32 {
	if b.State() == nil || b.State().SoH() == 0 {
		return float32(b.Capacity())
	}
	return (float32(b.Capacity()) * b.State().SoH()) / 100
}

func (b *Battery) ChargeDuration(chargePower float32, targetSoC float32) time.Duration {
	if targetSoC <= b.state.SoC() {
		return time.Duration(0)
	}
	// Charge power in watts
	power := min(chargePower, b.MaxChargePower())
	// State of charge
	soc := min(targetSoC, 100)
	// kWh total capacity
	kwhTotal := b.AvailableCapacity() * b.Voltage() / 1000
	// kWh requested to be put in the battery
	kwhChargeable := ((soc - b.State().SoC()) / 100) * kwhTotal
	// Watt seconds requested to be put in the battery
	wsChargeable := kwhChargeable * 3600000
	return time.Second * time.Duration(int64(wsChargeable)/int64(power))
}

func (b *Battery) NecessaryChargePower(targetSoC float32, targetChargeTime time.Duration) float32 {
	if targetSoC <= b.state.SoC() {
		return 0
	}
	// State fo charge
	soc := min(targetSoC, 100)
	// kWh total capacity
	kwhTotal := b.AvailableCapacity() * b.Voltage() / 1000
	// kWh requested to be put in the battery
	kwhChargeable := ((soc - b.State().SoC()) / 100) * kwhTotal
	// Watt seconds requested to be put in the battery
	wsChargeable := kwhChargeable * 3600000
	return wsChargeable / float32(targetChargeTime.Seconds())
}

func (b *Battery) StartMeasuring(context context.Context) {
	if b.updateTicker != nil {
		// Meter already started
		return
	}
	// Look for the highest update interval, phases and read line indices.
	interval := time.Millisecond
	meterPhases := uint8(0)
	readLineIndices := make([]uint8, 0)
	for _, meter := range b.meters {
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
	b.updateTicker = time.NewTicker(interval)
	go func() {
	loadLoop:
		for {
			select {
			case <-context.Done():
				for _, meter := range b.meters {
					meter.Shutdown()
				}
				return
			case _ = <-b.updateTicker.C:
				bs := battery.NewState()
				for _, meter := range b.meters {
					err := meter.UpdateValues(nil, nil, nil, nil, bs)
					if err != nil {
						log.Debugf("Failed to read battery values from energy meter with brand %s, model %s and serial %s: %s", meter.Brand(), meter.Model(), meter.Serial(), err)
						continue loadLoop
					}
				}
				b.state.SetValues(bs)
				if !bs.IsZero() {
					batteryMeterValues := events.NewBatteryMeterValues().
						SetName(b.Name()).
						SetRole(b.Role()).
						SetState(bs)
					events.BatteryMeterReadings.Trigger(batteryMeterValues)
				}
			}
		}
	}()
}

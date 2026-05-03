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
	voltage                float32
	chargingCoefficient    float32
	dischargingCoefficient float32
	state                  *battery.State
	meters                 []EnergyMeter
	updateTicker           *time.Ticker
	meteringStarted        bool
}

// NewBattery constructs a Battery with capacity (Ah), nominal voltage (V),
// and per-Ah charging / discharging C-rate coefficients (e.g. 0.5 for 0.5C).
func NewBattery(name string, capacity uint16, voltage, chargingCoefficient, dischargingCoefficient float32, meters []EnergyMeter) *Battery {
	if chargingCoefficient <= 0 {
		chargingCoefficient = 1
	}
	if dischargingCoefficient <= 0 {
		dischargingCoefficient = 1
	}
	return &Battery{
		name:                   name,
		capacity:               capacity,
		voltage:                voltage,
		chargingCoefficient:    chargingCoefficient,
		dischargingCoefficient: dischargingCoefficient,
		state:                  battery.NewState(),
		meters:                 meters,
	}
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

func (b *Battery) Voltage() float32 {
	return b.voltage
}

func (b *Battery) ChargingCoefficient() float32 {
	return b.chargingCoefficient
}

func (b *Battery) DischargingCoefficient() float32 {
	return b.dischargingCoefficient
}

func (b *Battery) State() *battery.State {
	return b.state
}

// MaxChargePowerW returns the maximum charge power in watts (capacity * coeff * voltage).
func (b *Battery) MaxChargePowerW() float32 {
	if b.capacity == 0 || b.voltage == 0 || b.chargingCoefficient == 0 {
		return 0
	}
	return float32(b.capacity) * b.chargingCoefficient * b.voltage
}

// MaxDischargePowerW returns the maximum discharge power in watts.
func (b *Battery) MaxDischargePowerW() float32 {
	if b.capacity == 0 || b.voltage == 0 || b.dischargingCoefficient == 0 {
		return 0
	}
	return float32(b.capacity) * b.dischargingCoefficient * b.voltage
}

// UsableCapacityKwh returns the usable battery energy in kWh, derated by State of Health.
func (b *Battery) UsableCapacityKwh() float32 {
	if b.capacity == 0 || b.voltage == 0 {
		return 0
	}
	soh := float32(100)
	if b.state != nil && b.state.SoH() > 0 {
		soh = b.state.SoH()
	}
	return float32(b.capacity) * b.voltage * soh / 100.0 / 1000.0
}

// IsMeasurementStarted reports whether the battery has produced at least one valid reading.
func (b *Battery) IsMeasurementStarted() bool {
	return b.meteringStarted
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
					b.meteringStarted = true
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

package domain

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type BatteryState struct {
	current float32
	power   float32
	voltage float32
	soc     float32
	soh     float32
}

type batteryState struct {
	Current float32 `json:"current" validate:"gte=0"`
	Power   float32 `json:"power"`
	Voltage float32 `json:"voltage" validate:"gte=0"`
	SoC     float32 `json:"soc" validate:"gte=0"`
	SoH     float32 `json:"soh" validate:"gte=0"`
}

func NewBatteryState() *BatteryState {
	return &BatteryState{}
}

func (bs *BatteryState) Current() float32 {
	return bs.current
}

func (bs *BatteryState) SetCurrent(current float32) {
	bs.current = current
}

func (bs *BatteryState) Power() float32 {
	return bs.power
}

func (bs *BatteryState) SetPower(power float32) {
	bs.power = power
}

func (bs *BatteryState) Voltage() float32 {
	return bs.voltage
}

func (bs *BatteryState) SetVoltage(voltage float32) {
	bs.voltage = voltage
}

func (bs *BatteryState) SoC() float32 {
	return bs.soc
}

func (bs *BatteryState) SetSoC(soc float32) {
	bs.soc = soc
}

func (bs *BatteryState) SoH() float32 {
	return bs.soh
}

func (bs *BatteryState) SetSoH(soh float32) {
	bs.soh = soh
}

func (bs *BatteryState) SetValues(other *BatteryState) {
	bs.current = other.current
	bs.power = other.power
	bs.voltage = other.voltage
	bs.soc = other.soc
	bs.soh = other.soh
}

func (bs *BatteryState) rawValues() batteryState {
	return batteryState{
		bs.current,
		bs.power,
		bs.voltage,
		bs.soc,
		bs.soh,
	}
}

func (bs *BatteryState) Valid() (bool, error) {
	rawValues := bs.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (bs *BatteryState) MarshalJSON() ([]byte, error) {
	rawValues := bs.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (bs *BatteryState) IsZero() bool {
	if bs.current != 0 {
		return false
	}
	if bs.power != 0 {
		return false
	}
	if bs.voltage != 0 {
		return false
	}
	return true
}

type Battery struct {
	name         string
	batteryState *BatteryState
	meters       []EnergyMeter
	updateTicker *time.Ticker
}

func (b *Battery) Name() string {
	return b.name
}

func (b *Battery) Role() EnergySourceRole {
	return RoleBattery
}

func (b *Battery) BatteryState() *BatteryState {
	return b.batteryState
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
		electricityMeter, ok := meter.(ElectricityMeter)
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
		for {
			select {
			case <-context.Done():
				for _, meter := range b.meters {
					meter.Shutdown()
				}
				return
			case _ = <-b.updateTicker.C:
				bs := NewBatteryState()
				for _, meter := range b.meters {
					meter.UpdateValues(nil, nil, nil, nil, bs)
				}
				b.batteryState = bs
				if !bs.IsZero() {
					batteryMeterValues := NewBatteryMeterValues().
						SetName(b.Name()).
						SetRole(b.Role()).
						SetBatteryState(bs)
					BatteryMeterReadings.Trigger(batteryMeterValues)
				}
			}
		}
	}()
}

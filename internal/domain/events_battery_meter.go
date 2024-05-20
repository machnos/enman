package domain

import (
	"errors"
	"time"
)

var BatteryMeterReadings = genericEventHandler[BatteryMeterValueChangeListener, *BatteryMeterValues]{
	listeners: make(map[BatteryMeterValueChangeListener]func(values *BatteryMeterValues) bool),
}

type BatteryMeterValueChangeListener interface {
	HandleEvent(*BatteryMeterValues)
}

type BatteryMeterValues struct {
	eventTime    time.Time
	name         string
	role         EnergySourceRole
	batteryState *BatteryState
}

func NewBatteryMeterValues() *BatteryMeterValues {
	return &BatteryMeterValues{
		eventTime: time.Now(),
	}
}

func (bmv *BatteryMeterValues) EventTime() time.Time {
	return bmv.eventTime
}

func (bmv *BatteryMeterValues) SetName(name string) *BatteryMeterValues {
	bmv.name = name
	return bmv
}

func (bmv *BatteryMeterValues) Name() string {
	return bmv.name
}

func (bmv *BatteryMeterValues) SetRole(role EnergySourceRole) *BatteryMeterValues {
	bmv.role = role
	return bmv
}

func (bmv *BatteryMeterValues) Role() EnergySourceRole {
	return bmv.role
}

func (bmv *BatteryMeterValues) SetBatteryState(batteryState *BatteryState) *BatteryMeterValues {
	bmv.batteryState = batteryState
	return bmv
}

func (bmv *BatteryMeterValues) BatteryState() *BatteryState {
	return bmv.batteryState
}

func (bmv *BatteryMeterValues) Valid() (bool, error) {
	var result error
	if bmv.batteryState != nil {
		_, err := bmv.batteryState.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

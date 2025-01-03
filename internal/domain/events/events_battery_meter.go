package events

import (
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
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
	eventTime time.Time
	name      string
	role      constants.EnergySourceRole
	state     *battery.State
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

func (bmv *BatteryMeterValues) SetRole(role constants.EnergySourceRole) *BatteryMeterValues {
	bmv.role = role
	return bmv
}

func (bmv *BatteryMeterValues) Role() constants.EnergySourceRole {
	return bmv.role
}

func (bmv *BatteryMeterValues) SetState(state *battery.State) *BatteryMeterValues {
	bmv.state = state
	return bmv
}

func (bmv *BatteryMeterValues) State() *battery.State {
	return bmv.state
}

func (bmv *BatteryMeterValues) Valid() (bool, error) {
	var result error
	if bmv.state != nil {
		_, err := bmv.state.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

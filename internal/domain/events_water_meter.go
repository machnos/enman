package domain

import (
	"errors"
	"time"
)

var WaterMeterReadings = genericEventHandler[WaterMeterValueChangeListener, *WaterMeterValues]{
	listeners: make(map[WaterMeterValueChangeListener]func(values *WaterMeterValues) bool),
}

type WaterMeterValueChangeListener interface {
	HandleEvent(*WaterMeterValues)
}

type WaterMeterValues struct {
	eventTime  time.Time
	name       string
	role       EnergySourceRole
	waterUsage *WaterUsage
}

func NewWaterMeterValues() *WaterMeterValues {
	return &WaterMeterValues{
		eventTime: time.Now(),
	}
}

func (wmv *WaterMeterValues) EventTime() time.Time {
	return wmv.eventTime
}

func (wmv *WaterMeterValues) SetName(name string) *WaterMeterValues {
	wmv.name = name
	return wmv
}

func (wmv *WaterMeterValues) Name() string {
	return wmv.name
}

func (wmv *WaterMeterValues) SetRole(role EnergySourceRole) *WaterMeterValues {
	wmv.role = role
	return wmv
}

func (wmv *WaterMeterValues) Role() EnergySourceRole {
	return wmv.role
}

func (wmv *WaterMeterValues) SetWaterUsage(waterUsage *WaterUsage) *WaterMeterValues {
	wmv.waterUsage = waterUsage
	return wmv
}

func (wmv *WaterMeterValues) WaterUsage() *WaterUsage {
	return wmv.waterUsage
}

func (wmv *WaterMeterValues) Valid() (bool, error) {
	var result error
	if wmv.waterUsage != nil {
		_, err := wmv.waterUsage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

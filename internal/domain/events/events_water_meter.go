package events

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/water"
	"errors"
	"time"
)

var WaterMeterReadings = genericEventHandler[WaterMeterValueChangeListener, *WaterMeterValues]{
	listeners: make(map[WaterMeterValueChangeListener]func(values *WaterMeterValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	WaterMeterReadings.SetContext(context.Background())
}

type WaterMeterValueChangeListener interface {
	HandleEvent(*WaterMeterValues)
}

type WaterMeterValues struct {
	eventTime time.Time
	name      string
	role      constants.EnergySourceRole
	usage     *water.Usage
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

func (wmv *WaterMeterValues) SetRole(role constants.EnergySourceRole) *WaterMeterValues {
	wmv.role = role
	return wmv
}

func (wmv *WaterMeterValues) Role() constants.EnergySourceRole {
	return wmv.role
}

func (wmv *WaterMeterValues) SetUsage(usage *water.Usage) *WaterMeterValues {
	wmv.usage = usage
	return wmv
}

func (wmv *WaterMeterValues) Usage() *water.Usage {
	return wmv.usage
}

func (wmv *WaterMeterValues) Valid() (bool, error) {
	var result error
	if wmv.usage != nil {
		_, err := wmv.usage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

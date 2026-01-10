package events

import (
	"context"
	"time"
)

var ChargingPeriodChanges = genericEventHandler[ChargingPeriodChangeListener, *ChargingPeriodValues]{
	listeners: make(map[ChargingPeriodChangeListener]func(values *ChargingPeriodValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	ChargingPeriodChanges.SetContext(context.Background())
}

type ChargingPeriodChangeListener interface {
	HandleEvent(*ChargingPeriodValues)
}

// ChargingPeriodValues represents an event for when a charging period starts or stops
type ChargingPeriodValues struct {
	eventTime  time.Time
	periodType ChargingPeriodEventType // Start or Stop
	startTime  time.Time
	endTime    time.Time
}

type ChargingPeriodEventType int

const (
	ChargingPeriodStart ChargingPeriodEventType = iota
	ChargingPeriodStop
)

func NewChargingPeriodValues() *ChargingPeriodValues {
	return &ChargingPeriodValues{
		eventTime: time.Now(),
	}
}

func (cpv *ChargingPeriodValues) EventTime() time.Time {
	return cpv.eventTime
}

func (cpv *ChargingPeriodValues) SetPeriodType(periodType ChargingPeriodEventType) *ChargingPeriodValues {
	cpv.periodType = periodType
	return cpv
}

func (cpv *ChargingPeriodValues) PeriodType() ChargingPeriodEventType {
	return cpv.periodType
}

func (cpv *ChargingPeriodValues) SetStartTime(startTime time.Time) *ChargingPeriodValues {
	cpv.startTime = startTime
	return cpv
}

func (cpv *ChargingPeriodValues) StartTime() time.Time {
	return cpv.startTime
}

func (cpv *ChargingPeriodValues) SetEndTime(endTime time.Time) *ChargingPeriodValues {
	cpv.endTime = endTime
	return cpv
}

func (cpv *ChargingPeriodValues) EndTime() time.Time {
	return cpv.endTime
}

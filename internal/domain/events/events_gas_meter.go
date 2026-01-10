package events

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/gas"
	"errors"
	"time"
)

var GasMeterReadings = genericEventHandler[GasMeterValueChangeListener, *GasMeterValues]{
	listeners: make(map[GasMeterValueChangeListener]func(values *GasMeterValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	GasMeterReadings.SetContext(context.Background())
}

type GasMeterValueChangeListener interface {
	HandleEvent(*GasMeterValues)
}

type GasMeterValues struct {
	eventTime time.Time
	name      string
	role      constants.EnergySourceRole
	usage     *gas.Usage
}

func NewGasMeterValues() *GasMeterValues {
	return &GasMeterValues{
		eventTime: time.Now(),
	}
}

func (gmv *GasMeterValues) EventTime() time.Time {
	return gmv.eventTime
}

func (gmv *GasMeterValues) SetName(name string) *GasMeterValues {
	gmv.name = name
	return gmv
}

func (gmv *GasMeterValues) Name() string {
	return gmv.name
}

func (gmv *GasMeterValues) SetRole(role constants.EnergySourceRole) *GasMeterValues {
	gmv.role = role
	return gmv
}

func (gmv *GasMeterValues) Role() constants.EnergySourceRole {
	return gmv.role
}

func (gmv *GasMeterValues) SetUsage(usage *gas.Usage) *GasMeterValues {
	gmv.usage = usage
	return gmv
}

func (gmv *GasMeterValues) Usage() *gas.Usage {
	return gmv.usage
}

func (gmv *GasMeterValues) Valid() (bool, error) {
	var result error
	if gmv.usage != nil {
		_, err := gmv.usage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

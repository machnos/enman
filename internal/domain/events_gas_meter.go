package domain

import (
	"errors"
	"time"
)

var GasMeterReadings = genericEventHandler[GasMeterValueChangeListener, *GasMeterValues]{
	listeners: make(map[GasMeterValueChangeListener]func(values *GasMeterValues) bool),
}

type GasMeterValueChangeListener interface {
	HandleEvent(*GasMeterValues)
}

type GasMeterValues struct {
	eventTime time.Time
	name      string
	role      EnergySourceRole
	gasUsage  *GasUsage
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

func (gmv *GasMeterValues) SetRole(role EnergySourceRole) *GasMeterValues {
	gmv.role = role
	return gmv
}

func (gmv *GasMeterValues) Role() EnergySourceRole {
	return gmv.role
}

func (gmv *GasMeterValues) SetGasUsage(gasUsage *GasUsage) *GasMeterValues {
	gmv.gasUsage = gasUsage
	return gmv
}

func (gmv *GasMeterValues) GasUsage() *GasUsage {
	return gmv.gasUsage
}

func (gmv *GasMeterValues) Valid() (bool, error) {
	var result error
	if gmv.gasUsage != nil {
		_, err := gmv.gasUsage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

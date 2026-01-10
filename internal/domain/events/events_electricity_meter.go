package events

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"errors"
	"time"
)

var ElectricityMeterReadings = genericEventHandler[ElectricityMeterValueChangeListener, *ElectricityMeterValues]{
	listeners: make(map[ElectricityMeterValueChangeListener]func(values *ElectricityMeterValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	ElectricityMeterReadings.SetContext(context.Background())
}

type ElectricityMeterValueChangeListener interface {
	HandleEvent(*ElectricityMeterValues)
}

type ElectricityMeterValues struct {
	eventTime       time.Time
	name            string
	role            constants.EnergySourceRole
	meterPhases     uint8
	readLineIndices []uint8
	state           *electricity.State
	usage           *electricity.Usage
}

func NewElectricityMeterValues() *ElectricityMeterValues {
	return &ElectricityMeterValues{
		eventTime: time.Now(),
	}
}

func (emv *ElectricityMeterValues) EventTime() time.Time {
	return emv.eventTime
}

func (emv *ElectricityMeterValues) SetName(name string) *ElectricityMeterValues {
	emv.name = name
	return emv
}

func (emv *ElectricityMeterValues) Name() string {
	return emv.name
}

func (emv *ElectricityMeterValues) SetRole(role constants.EnergySourceRole) *ElectricityMeterValues {
	emv.role = role
	return emv
}

func (emv *ElectricityMeterValues) Role() constants.EnergySourceRole {
	return emv.role
}

func (emv *ElectricityMeterValues) SetMeterPhases(meterPhases uint8) *ElectricityMeterValues {
	emv.meterPhases = meterPhases
	return emv
}

func (emv *ElectricityMeterValues) MeterPhases() uint8 {
	return emv.meterPhases
}

func (emv *ElectricityMeterValues) SetReadLineIndices(readLineIndices []uint8) *ElectricityMeterValues {
	emv.readLineIndices = readLineIndices
	return emv
}

func (emv *ElectricityMeterValues) ReadLineIndices() []uint8 {
	return emv.readLineIndices
}

func (emv *ElectricityMeterValues) SetState(state *electricity.State) *ElectricityMeterValues {
	emv.state = state
	return emv
}

func (emv *ElectricityMeterValues) State() *electricity.State {
	return emv.state
}

func (emv *ElectricityMeterValues) SetUsage(usage *electricity.Usage) *ElectricityMeterValues {
	emv.usage = usage
	return emv
}

func (emv *ElectricityMeterValues) Usage() *electricity.Usage {
	return emv.usage
}

func (emv *ElectricityMeterValues) Valid() (bool, error) {
	var result error
	if emv.state != nil {
		_, err := emv.state.Valid()
		result = errors.Join(result, err)
	}
	if emv.usage != nil {
		_, err := emv.usage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

package domain

import (
	"errors"
	"time"
)

var ElectricityMeterReadings = genericEventHandler[ElectricityMeterValueChangeListener, *ElectricityMeterValues]{
	listeners: make(map[ElectricityMeterValueChangeListener]func(values *ElectricityMeterValues) bool),
}

type ElectricityMeterValueChangeListener interface {
	HandleEvent(*ElectricityMeterValues)
}

type ElectricityMeterValues struct {
	eventTime        time.Time
	name             string
	role             ElectricitySourceRole
	meterBrand       string
	meterType        string
	meterSerial      string
	meterPhases      uint8
	readLineIndices  []uint8
	electricityUsage *ElectricityUsage
	electricityState *ElectricityState
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

func (emv *ElectricityMeterValues) SetRole(role ElectricitySourceRole) *ElectricityMeterValues {
	emv.role = role
	return emv
}

func (emv *ElectricityMeterValues) Role() ElectricitySourceRole {
	return emv.role
}

func (emv *ElectricityMeterValues) SetMeterBrand(meterBrand string) *ElectricityMeterValues {
	emv.meterBrand = meterBrand
	return emv
}

func (emv *ElectricityMeterValues) MeterBrand() string {
	return emv.meterBrand
}

func (emv *ElectricityMeterValues) SetMeterType(meterType string) *ElectricityMeterValues {
	emv.meterType = meterType
	return emv
}

func (emv *ElectricityMeterValues) MeterType() string {
	return emv.meterType
}

func (emv *ElectricityMeterValues) SetMeterSerial(meterSerial string) *ElectricityMeterValues {
	emv.meterSerial = meterSerial
	return emv
}

func (emv *ElectricityMeterValues) MeterSerial() string {
	return emv.meterSerial
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

func (emv *ElectricityMeterValues) SetElectricityState(electricityState *ElectricityState) *ElectricityMeterValues {
	emv.electricityState = electricityState
	return emv
}

func (emv *ElectricityMeterValues) ElectricityState() *ElectricityState {
	return emv.electricityState
}

func (emv *ElectricityMeterValues) SetElectricityUsage(electricityUsage *ElectricityUsage) *ElectricityMeterValues {
	emv.electricityUsage = electricityUsage
	return emv
}

func (emv *ElectricityMeterValues) ElectricityUsage() *ElectricityUsage {
	return emv.electricityUsage
}

func (emv *ElectricityMeterValues) Valid() (bool, error) {
	var result error
	if emv.electricityState != nil {
		_, err := emv.electricityState.Valid()
		result = errors.Join(result, err)
	}
	if emv.electricityUsage != nil {
		_, err := emv.electricityUsage.Valid()
		result = errors.Join(result, err)
	}
	return result == nil, result
}

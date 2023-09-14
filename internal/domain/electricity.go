package domain

import (
	"context"
	"encoding/json"
)

const (
	/*
		// MinVoltage The minimum voltage a grid must have.
		MinVoltage uint16 = 1
		// MaxVoltage the maximum voltage a grid may have.
		MaxVoltage uint16 = 600
		// MinCurrentPerPhase The minimum current per phase a grid must have.
		MinCurrentPerPhase float32 = 0.1
		// MaxCurrentPerPhase The maximum current per phase a grid may have.
		MaxCurrentPerPhase float32 = 100
	*/
	// MinPhases The minimum number of phases a grid must have
	MinPhases uint8 = 1
	// MaxPhases The maximum number of phases a grid may have
	MaxPhases uint8 = 3
)

type ElectricityMeter interface {
	Name() string
	Role() ElectricitySourceRole
	StartReading(context.Context) ElectricityMeter
	// WaitForInitialization waits until the electricity meter is fully initialized and returns true or false
	// whether the meter is capable of reading values.
	WaitForInitialization() bool
	LineIndices() []uint8
}

type ElectricityUsage struct {
	energyConsumed      [MaxPhases]float64
	totalEnergyConsumed float64
	energyProvided      [MaxPhases]float64
	totalEnergyProvided float64
}

type electricityUsage struct {
	EnergyConsumed      [MaxPhases]float64 `json:"energy_consumed" validate:"dive,gte=0"`
	TotalEnergyConsumed float64            `json:"total_energy_consumed" validate:"gte=0"`
	EnergyProvided      [MaxPhases]float64 `json:"energy_provided" validate:"dive,gte=0"`
	TotalEnergyProvided float64            `json:"total_energy_provided" validate:"gte=0"`
}

func NewElectricityUsage() *ElectricityUsage {
	return &ElectricityUsage{
		energyConsumed: [3]float64(make([]float64, MaxPhases)),
		energyProvided: [3]float64(make([]float64, MaxPhases)),
	}
}

func (eu *ElectricityUsage) EnergyConsumed(lineIx uint8) float64 {
	return eu.energyConsumed[lineIx]
}

func (eu *ElectricityUsage) SetEnergyConsumed(lineIx uint8, energyConsumed float64) *ElectricityUsage {
	eu.energyConsumed[lineIx] = energyConsumed
	return eu
}

func (eu *ElectricityUsage) SetTotalEnergyConsumed(totalEnergyConsumed float64) *ElectricityUsage {
	eu.totalEnergyConsumed = totalEnergyConsumed
	return eu
}

func (eu *ElectricityUsage) TotalEnergyConsumed() float64 {
	if eu.totalEnergyConsumed != 0 {
		return eu.totalEnergyConsumed
	}
	totalEnergyConsumed := float64(0)
	for i := 0; i < len(eu.energyConsumed); i++ {
		totalEnergyConsumed += eu.energyConsumed[i]
	}
	return totalEnergyConsumed
}

func (eu *ElectricityUsage) EnergyProvided(lineIx uint8) float64 {
	return eu.energyProvided[lineIx]
}

func (eu *ElectricityUsage) SetEnergyProvided(lineIx uint8, energyProvided float64) *ElectricityUsage {
	eu.energyProvided[lineIx] = energyProvided
	return eu
}

func (eu *ElectricityUsage) SetTotalEnergyProvided(totalEnergyProvided float64) *ElectricityUsage {
	eu.totalEnergyProvided = totalEnergyProvided
	return eu
}

func (eu *ElectricityUsage) TotalEnergyProvided() float64 {
	if eu.totalEnergyProvided != 0 {
		return eu.totalEnergyProvided
	}
	totalEnergyProvided := float64(0)
	for i := 0; i < len(eu.energyProvided); i++ {
		totalEnergyProvided += eu.energyProvided[i]
	}
	return totalEnergyProvided
}

func (eu *ElectricityUsage) IsZero() bool {
	if eu.totalEnergyConsumed != 0 || eu.totalEnergyProvided != 0 {
		return false
	}
	for _, value := range eu.energyConsumed {
		if value != 0 {
			return false
		}
	}
	for _, value := range eu.energyProvided {
		if value != 0 {
			return false
		}
	}
	return true
}

func (eu *ElectricityUsage) SetValues(other *ElectricityUsage) {
	eu.energyConsumed = other.energyConsumed
	eu.energyProvided = other.energyProvided
	eu.totalEnergyConsumed = other.totalEnergyConsumed
	eu.totalEnergyProvided = other.totalEnergyProvided
}

func (eu *ElectricityUsage) rawValues() electricityUsage {
	return electricityUsage{eu.energyConsumed,
		eu.TotalEnergyConsumed(),
		eu.energyProvided,
		eu.TotalEnergyProvided()}
}

func (eu *ElectricityUsage) Valid() (bool, error) {
	err := validator.Struct(eu.rawValues())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (eu *ElectricityUsage) MarshalJSON() ([]byte, error) {
	rawValues := eu.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

type ElectricityState struct {
	current [MaxPhases]float32
	power   [MaxPhases]float32
	voltage [MaxPhases]float32
}

type electricityState struct {
	Current      [MaxPhases]float32 `json:"current"`
	TotalCurrent float32            `json:"total_current"`
	Power        [MaxPhases]float32 `json:"power"`
	TotalPower   float32            `json:"total_power"`
	Voltage      [MaxPhases]float32 `json:"voltage" validate:"dive,gte=0"`
}

func NewElectricityState() *ElectricityState {
	return &ElectricityState{
		[MaxPhases]float32(make([]float32, MaxPhases)),
		[MaxPhases]float32(make([]float32, MaxPhases)),
		[MaxPhases]float32(make([]float32, MaxPhases)),
	}
}

func (es *ElectricityState) Power(lineIx uint8) float32 {
	return es.power[lineIx]
}

func (es *ElectricityState) SetPower(lineIx uint8, power float32) *ElectricityState {
	es.power[lineIx] = power
	return es
}

func (es *ElectricityState) TotalPower() float32 {
	totalPower := float32(0)
	for i := 0; i < len(es.power); i++ {
		totalPower += es.power[i]
	}
	return totalPower
}

func (es *ElectricityState) Voltage(lineIx uint8) float32 {
	return es.voltage[lineIx]
}

func (es *ElectricityState) SetVoltage(lineIx uint8, voltage float32) *ElectricityState {
	es.voltage[lineIx] = voltage
	return es
}

func (es *ElectricityState) Current(lineIx uint8) float32 {
	return es.current[lineIx]
}

func (es *ElectricityState) SetCurrent(lineIx uint8, current float32) *ElectricityState {
	es.current[lineIx] = current
	return es
}

func (es *ElectricityState) TotalCurrent() float32 {
	totalCurrent := float32(0)
	for i := 0; i < len(es.current); i++ {
		totalCurrent += es.current[i]
	}
	return totalCurrent
}

func (es *ElectricityState) Phases() uint8 {
	for x := MaxPhases; x >= MinPhases; x-- {
		if es.voltage[x-1] != 0 {
			return x
		}
	}
	return 0
}

func (es *ElectricityState) SetValues(other *ElectricityState) {
	es.power = other.power
	es.current = other.current
	es.voltage = other.voltage
}

func (es *ElectricityState) rawValues() electricityState {
	return electricityState{es.current,
		es.TotalCurrent(),
		es.power,
		es.TotalPower(),
		es.voltage}
}

func (es *ElectricityState) Valid() (bool, error) {
	rawValues := es.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (es *ElectricityState) MarshalJSON() ([]byte, error) {
	rawValues := es.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (es *ElectricityState) IsZero() bool {
	for voltage := range es.voltage {
		if voltage != 0 {
			return false
		}
	}
	for current := range es.current {
		if current != 0 {
			return false
		}
	}
	for power := range es.power {
		if power != 0 {
			return false
		}
	}
	return true
}

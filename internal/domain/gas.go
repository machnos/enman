package domain

import "encoding/json"

type GasMeter interface {
	EnergyMeter
	IsGasMeter() bool
}

type GasUsage struct {
	gasConsumed float64
}

type gasUsage struct {
	GasConsumed float64 `json:"gas_consumed" validate:"gte=0"`
}

func NewGasUsage() *GasUsage {
	return &GasUsage{}
}

func (gu *GasUsage) GasConsumed() float64 {
	return gu.gasConsumed
}

func (gu *GasUsage) SetGasConsumed(gasConsumed float64) {
	gu.gasConsumed = gasConsumed
}

func (gu *GasUsage) SetValues(other *GasUsage) {
	gu.gasConsumed = other.gasConsumed
}

func (gu *GasUsage) rawValues() gasUsage {
	return gasUsage{gu.gasConsumed}
}

func (gu *GasUsage) Valid() (bool, error) {
	rawValues := gu.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (gu *GasUsage) MarshalJSON() ([]byte, error) {
	rawValues := gu.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (gu *GasUsage) IsZero() bool {
	if gu.gasConsumed != 0 {
		return false
	}
	return true
}

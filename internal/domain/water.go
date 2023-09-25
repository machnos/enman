package domain

import "encoding/json"

type WaterMeter interface {
	EnergyMeter
	IsWaterMeter() bool
}

type WaterUsage struct {
	waterConsumed float64
}

type waterUsage struct {
	WaterConsumed float64 `json:"water_consumed" validate:"gte=0"`
}

func NewWaterUsage() *WaterUsage {
	return &WaterUsage{}
}

func (wu *WaterUsage) WaterConsumed() float64 {
	return wu.waterConsumed
}

func (wu *WaterUsage) SetWaterConsumed(waterConsumed float64) {
	wu.waterConsumed = waterConsumed
}

func (wu *WaterUsage) SetValues(other *WaterUsage) {
	wu.waterConsumed = other.waterConsumed
}

func (wu *WaterUsage) rawValues() waterUsage {
	return waterUsage{wu.waterConsumed}
}

func (wu *WaterUsage) Valid() (bool, error) {
	rawValues := wu.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (wu *WaterUsage) MarshalJSON() ([]byte, error) {
	rawValues := wu.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (wu *WaterUsage) IsZero() bool {
	if wu.waterConsumed != 0 {
		return false
	}
	return true
}

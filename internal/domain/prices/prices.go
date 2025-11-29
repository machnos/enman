package prices

import (
	"fmt"
	"strings"
	"time"
)

type EnergyType uint64

const (
	EnergyTypeNone EnergyType = iota
	EnergyTypeElectricity
	EnergyTypeGas
	EnergyTypeWater
)

func (e EnergyType) String() string {
	switch e {
	case EnergyTypeElectricity:
		return "electricity"
	case EnergyTypeGas:
		return "gas"
	case EnergyTypeWater:
		return "water"
	default:
		return fmt.Sprintf("unknown EnergyType (%d)", e)
	}
}

func ParseEnergyType(energyType string) (EnergyType, error) {
	switch strings.ToLower(energyType) {
	case "electricity":
		return EnergyTypeElectricity, nil
	case "gas":
		return EnergyTypeGas, nil
	case "water":
		return EnergyTypeWater, nil
	default:
		return EnergyTypeNone, fmt.Errorf("invalid EnergyType: %s", energyType)
	}
}

type EnergyPriceProvider struct {
	Name        string
	EnergyTypes []EnergyType
}

func (epp *EnergyPriceProvider) EnergyTypesAsStrings() []string {
	types := make([]string, 0)
	for _, energyType := range epp.EnergyTypes {
		types = append(types, energyType.String())
	}
	return types
}

type EnergyPrice struct {
	Time             time.Time
	EndTime          time.Time
	ProviderName     string
	EnergyType       EnergyType
	ConsumptionPrice float32
	FeedbackPrice    float32
}

func (ep *EnergyPrice) Duration() time.Duration {
	return ep.EndTime.Sub(ep.Time)
}

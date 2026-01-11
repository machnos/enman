package domain

import (
	"enman/internal/domain/repository"
)

type PriceBasedElectricityConsumptionCalculator struct {
	repository   repository.EnergyPrice
	providerName string
	batteries    []*Battery
}

func NewPriceBasedElectricityConsumptionCalculator(
	repository repository.EnergyPrice,
	providerName string,
	batteries []*Battery,
) *PriceBasedElectricityConsumptionCalculator {
	return &PriceBasedElectricityConsumptionCalculator{
		repository:   repository,
		providerName: providerName,
		batteries:    batteries,
	}
}

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) float32 {
	return 0
}

package domain

import (
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"sort"
	"time"
)

type PriceBasedElectricityConsumptionCalculator struct {
	repository   repository.EnergyPrice
	providerName string
}

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition() int {
	energyPrices, err := p.repository.EnergyPrices(time.Now(), time.Time{}, p.providerName, prices.EnergyTypeElectricity)
	if err != nil {
		return 0
	}
	sort.Slice(energyPrices, func(i, j int) bool {
		return energyPrices[i].ConsumptionPrice < energyPrices[j].ConsumptionPrice
	})

	return 0
}

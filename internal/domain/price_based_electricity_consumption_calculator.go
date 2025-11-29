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
	batteries    []*Battery
}

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) int {
	energyPrices, err := p.repository.EnergyPrices(time.Now(), time.Time{}, p.providerName, prices.EnergyTypeElectricity)
	if err != nil || len(energyPrices) < 2 {
		return 0
	}
	sort.Slice(energyPrices, func(i, j int) bool {
		return energyPrices[i].ConsumptionPrice < energyPrices[j].ConsumptionPrice
	})
	totalAddition := 0
	totalChargePower := availableChargePower
	now := time.Now()
	for _, b := range p.batteries {
		if totalChargePower <= 0 {
			break
		}
		duration, batteryChargePower := b.ChargeDuration(totalChargePower, 95)
		for _, price := range energyPrices {
			if duration.Minutes() < 1 {
				break
			}

			if price.Time.Before(now) && price.EndTime.After(now) {
				totalAddition += int(batteryChargePower)
				totalChargePower -= batteryChargePower
				break
			}
			duration -= price.Duration()
		}
	}
	return totalAddition
}

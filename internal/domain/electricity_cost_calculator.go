package domain

import (
	"enman/internal/log"
	"fmt"
	"time"
)

type ElectricityUsageCostCalculator struct {
	repository     Repository
	previousValues map[string]*ElectricityPriceValues
}

func NewElectricityUsageCostCalculator(repository Repository) *ElectricityUsageCostCalculator {
	calculator := &ElectricityUsageCostCalculator{
		repository:     repository,
		previousValues: make(map[string]*ElectricityPriceValues),
	}
	return calculator
}

func (e *ElectricityUsageCostCalculator) HandleEvent(values *ElectricityPriceValues) {
	cacheKey := fmt.Sprintf("%s", values.EnergyProviderName())
	defer func() { e.previousValues[cacheKey] = values }()
	var startTime time.Time
	previousConsumptionPrice := float32(0)
	previousFeedbackPrice := float32(0)
	endTime := values.PriceStartingTime()
	if value, ok := e.previousValues[cacheKey]; ok {
		startTime = value.PriceStartingTime()
		previousConsumptionPrice = value.ConsumptionPrice()
		previousFeedbackPrice = value.FeedbackPrice()
	} else {
		dbPrice, err := e.repository.EnergyPriceAtTime(values.PriceStartingTime().Add(time.Minute*-1), values.EnergyProviderName(), LessOrEqual)
		if err != nil {
			log.Errorf("Unable to determine previous electricity price: %s", err.Error())
			return
		}
		if dbPrice == nil {
			log.Warning("Unable to determine previous electricity price as it is not found in the database")
			return
		}
		startTime = dbPrice.Time
		previousConsumptionPrice = dbPrice.ConsumptionPrice
		previousFeedbackPrice = dbPrice.FeedbackPrice
	}
	startUsage, err := e.repository.ElectricityUsageAtTime(startTime, "", RoleGrid, EqualOrGreater)
	if err != nil {
		log.Errorf("Unable to determine start usage: %s", err.Error())
		return
	}
	endUsage, err := e.repository.ElectricityUsageAtTime(endTime, "", RoleGrid, LessOrEqual)
	if err != nil {
		log.Errorf("Unable to determine end usage: %s", err.Error())
		return
	}
	valuesEvent := NewElectricityCostsValues().
		SetStartTime(startTime).
		SetEndTime(endTime).
		SetEnergyProviderName(values.energyProviderName).
		SetConsumptionPricePerKwh(previousConsumptionPrice).
		SetStartConsumptionEnergy(startUsage.ElectricityUsage.TotalEnergyConsumed()).
		SetEndConsumptionEnergy(endUsage.ElectricityUsage.TotalEnergyConsumed()).
		SetFeedbackPricePerKwh(previousFeedbackPrice).
		SetStartFeedbackEnergy(startUsage.ElectricityUsage.TotalEnergyProvided()).
		SetEndFeedbackEnergy(endUsage.ElectricityUsage.TotalEnergyProvided())
	ElectricityCosts.Trigger(valuesEvent)
}

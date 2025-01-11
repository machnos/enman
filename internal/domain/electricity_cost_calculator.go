package domain

import (
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"sync"
	"time"
)

type ElectricityUsageCostCalculator struct {
	repository     repository.Repository
	previousValues sync.Map
}

func NewElectricityUsageCostCalculator(repository repository.Repository) *ElectricityUsageCostCalculator {
	calculator := &ElectricityUsageCostCalculator{
		repository: repository,
	}
	return calculator
}

func (e *ElectricityUsageCostCalculator) HandleEvent(values *events.EnergyPriceValues) {
	cacheKey := values.EnergyProviderName()
	defer func() { e.previousValues.Store(cacheKey, values) }()
	var startTime time.Time
	previousConsumptionPrice := float32(0)
	previousFeedbackPrice := float32(0)
	endTime := values.PriceStartingTime()
	if value, ok := e.previousValues.Load(cacheKey); ok {
		previousValue := value.(*events.EnergyPriceValues)
		startTime = previousValue.PriceStartingTime()
		previousConsumptionPrice = previousValue.ConsumptionPrice()
		previousFeedbackPrice = previousValue.FeedbackPrice()
	} else {
		dbPrice, err := e.repository.EnergyPriceAtTime(values.PriceStartingTime().Add(time.Minute*-1), values.EnergyProviderName(), prices.EnergyTypeElectricity, repository.LessOrEqual)
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
	startUsage, err := e.repository.ElectricityUsageAtTime(startTime, "", constants.EnergySourceRoleGrid, repository.EqualOrGreater)
	if err != nil {
		log.Errorf("Unable to determine start usage: %s", err.Error())
		return
	} else if startUsage == nil {
		log.Warning("Unable to determine start usage")
		return
	}
	endUsage, err := e.repository.ElectricityUsageAtTime(endTime, "", constants.EnergySourceRoleGrid, repository.LessOrEqual)
	if err != nil {
		log.Errorf("Unable to determine end usage: %s", err.Error())
		return
	} else if endUsage == nil {
		log.Warning("Unable to determine end usage")
		return
	}
	valuesEvent := events.NewElectricityCostsValues().
		SetStartTime(startTime).
		SetEndTime(endTime).
		SetEnergyProviderName(values.EnergyProviderName()).
		SetConsumptionPricePerKwh(previousConsumptionPrice).
		SetStartConsumptionEnergy(startUsage.TotalEnergyConsumed()).
		SetEndConsumptionEnergy(endUsage.TotalEnergyConsumed()).
		SetFeedbackPricePerKwh(previousFeedbackPrice).
		SetStartFeedbackEnergy(startUsage.TotalEnergyProvided()).
		SetEndFeedbackEnergy(endUsage.TotalEnergyProvided())
	events.ElectricityCosts.Trigger(valuesEvent)
}

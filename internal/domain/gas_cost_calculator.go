package domain

import (
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/log"
	"sync"
	"time"
)

type GasUsageCostCalculator struct {
	repository     Repository
	previousValues sync.Map
}

func NewGasUsageCostCalculator(repository Repository) *GasUsageCostCalculator {
	calculator := &GasUsageCostCalculator{
		repository: repository,
	}
	return calculator
}

func (e *GasUsageCostCalculator) HandleEvent(values *events.EnergyPriceValues) {
	cacheKey := values.EnergyProviderName()
	defer func() { e.previousValues.Store(cacheKey, values) }()
	var startTime time.Time
	previousConsumptionPrice := float32(0)
	endTime := values.PriceStartingTime()
	if value, ok := e.previousValues.Load(cacheKey); ok {
		previousValue := value.(*events.EnergyPriceValues)
		startTime = previousValue.PriceStartingTime()
		previousConsumptionPrice = previousValue.ConsumptionPrice()
	} else {
		dbPrice, err := e.repository.EnergyPriceAtTime(values.PriceStartingTime().Add(time.Minute*-1), values.EnergyProviderName(), prices.EnergyTypeGas, LessOrEqual)
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
	}
	startUsage, err := e.repository.GasUsageAtTime(startTime, "", constants.EnergySourceRoleGrid, EqualOrGreater)
	if err != nil {
		log.Errorf("Unable to determine start usage: %s", err.Error())
		return
	} else if startUsage == nil {
		log.Warning("Unable to determine start usage")
		return
	}
	endUsage, err := e.repository.GasUsageAtTime(endTime, "", constants.EnergySourceRoleGrid, LessOrEqual)
	if err != nil {
		log.Errorf("Unable to determine end usage: %s", err.Error())
		return
	} else if endUsage == nil {
		log.Warning("Unable to determine end usage")
		return
	}
	valuesEvent := events.NewGasCostsValues().
		SetStartTime(startTime).
		SetEndTime(endTime).
		SetEnergyProviderName(values.EnergyProviderName()).
		SetConsumptionPricePerM3(previousConsumptionPrice).
		SetStartConsumptionUsage(startUsage.GasConsumed()).
		SetEndConsumptionUsage(endUsage.GasConsumed())
	events.GasCosts.Trigger(valuesEvent)
}

package events

import (
	"context"
	"time"
)

var GasCosts = genericEventHandler[GasCostsChangeListener, *GasCostsValues]{
	listeners: make(map[GasCostsChangeListener]func(values *GasCostsValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	GasCosts.SetContext(context.Background())
}

type GasCostsChangeListener interface {
	HandleEvent(*GasCostsValues)
}

type GasCostsValues struct {
	eventTime             time.Time
	startTime             time.Time
	endTime               time.Time
	startConsumptionUsage float64
	endConsumptionUsage   float64
	energyProviderName    string
	consumptionPricePerM3 float32
}

func NewGasCostsValues() *GasCostsValues {
	return &GasCostsValues{
		eventTime: time.Now(),
	}
}

func (ecv *GasCostsValues) EventTime() time.Time {
	return ecv.eventTime
}

func (ecv *GasCostsValues) StartTime() time.Time {
	return ecv.startTime
}

func (ecv *GasCostsValues) SetStartTime(startTime time.Time) *GasCostsValues {
	ecv.startTime = startTime
	return ecv
}

func (ecv *GasCostsValues) EndTime() time.Time {
	return ecv.endTime
}

func (ecv *GasCostsValues) SetEndTime(endTime time.Time) *GasCostsValues {
	ecv.endTime = endTime
	return ecv
}

func (ecv *GasCostsValues) StartConsumptionUsage() float64 {
	return ecv.startConsumptionUsage
}

func (ecv *GasCostsValues) SetStartConsumptionUsage(startConsumptionUsage float64) *GasCostsValues {
	ecv.startConsumptionUsage = startConsumptionUsage
	return ecv
}

func (ecv *GasCostsValues) EndConsumptionUsage() float64 {
	return ecv.endConsumptionUsage
}

func (ecv *GasCostsValues) SetEndConsumptionUsage(endConsumptionUsage float64) *GasCostsValues {
	ecv.endConsumptionUsage = endConsumptionUsage
	return ecv
}

func (ecv *GasCostsValues) ConsumptionUsage() float32 {
	return float32(ecv.EndConsumptionUsage() - ecv.StartConsumptionUsage())
}

func (ecv *GasCostsValues) EnergyProviderName() string {
	return ecv.energyProviderName
}

func (ecv *GasCostsValues) SetEnergyProviderName(energyProviderName string) *GasCostsValues {
	ecv.energyProviderName = energyProviderName
	return ecv
}

func (ecv *GasCostsValues) ConsumptionPricePerM3() float32 {
	return ecv.consumptionPricePerM3
}

func (ecv *GasCostsValues) SetConsumptionPricePerM3(consumptionPricePerM3 float32) *GasCostsValues {
	ecv.consumptionPricePerM3 = consumptionPricePerM3
	return ecv
}

func (ecv *GasCostsValues) ConsumptionCosts() float32 {
	return ecv.ConsumptionPricePerM3() * ecv.ConsumptionUsage()
}

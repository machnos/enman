package events

import (
	"context"
	"enman/internal/domain/prices"
	"time"
)

var EnergyPrices = genericEventHandler[EnergyPriceChangeListener, *EnergyPriceValues]{
	listeners: make(map[EnergyPriceChangeListener]func(values *EnergyPriceValues) bool),
}

func init() {
	// Initialize with a background context - will be set by application during startup
	EnergyPrices.SetContext(context.Background())
}

type EnergyPriceChangeListener interface {
	HandleEvent(*EnergyPriceValues)
}

type EnergyPriceValues struct {
	eventTime          time.Time
	energyProviderName string
	energyType         prices.EnergyType
	priceStartTime     time.Time
	priceEndTime       time.Time
	consumptionPrice   float32
	feedbackPrice      float32
	interval           time.Duration
}

func NewEnergyPriceValues() *EnergyPriceValues {
	return &EnergyPriceValues{
		eventTime: time.Now(),
	}
}

func (epv *EnergyPriceValues) EventTime() time.Time {
	return epv.eventTime
}

func (epv *EnergyPriceValues) SetEnergyProviderName(energyProviderName string) *EnergyPriceValues {
	epv.energyProviderName = energyProviderName
	return epv
}

func (epv *EnergyPriceValues) EnergyProviderName() string {
	return epv.energyProviderName
}

func (evp *EnergyPriceValues) SetEnergyType(energyType prices.EnergyType) *EnergyPriceValues {
	evp.energyType = energyType
	return evp
}

func (evp *EnergyPriceValues) EnergyType() prices.EnergyType {
	return evp.energyType
}

func (epv *EnergyPriceValues) SetConsumptionPrice(consumptionPrice float32) *EnergyPriceValues {
	epv.consumptionPrice = consumptionPrice
	return epv
}

func (epv *EnergyPriceValues) ConsumptionPrice() float32 {
	return epv.consumptionPrice
}

func (epv *EnergyPriceValues) SetFeedbackPrice(feedbackPrice float32) *EnergyPriceValues {
	epv.feedbackPrice = feedbackPrice
	return epv
}

func (epv *EnergyPriceValues) FeedbackPrice() float32 {
	return epv.feedbackPrice
}

func (epv *EnergyPriceValues) SetPriceStartTime(priceStartTime time.Time) *EnergyPriceValues {
	epv.priceStartTime = priceStartTime
	return epv
}

func (epv *EnergyPriceValues) PriceStartTime() time.Time {
	return epv.priceStartTime
}

func (epv *EnergyPriceValues) SetPriceEndTime(priceEndTime time.Time) *EnergyPriceValues {
	epv.priceEndTime = priceEndTime
	return epv
}

func (epv *EnergyPriceValues) PriceEndingTime() time.Time {
	return epv.priceEndTime
}

package domain

import (
	"time"
)

var ElectricityPrices = genericEventHandler[ElectricityPriceChangeListener, *ElectricityPriceValues]{
	listeners: make(map[ElectricityPriceChangeListener]func(values *ElectricityPriceValues) bool),
}

type ElectricityPriceChangeListener interface {
	HandleEvent(*ElectricityPriceValues)
}

type ElectricityPriceValues struct {
	eventTime          time.Time
	energyProviderName string
	priceStartingTime  time.Time
	consumptionPrice   float32
	feedbackPrice      float32
	interval           time.Duration
}

func NewElectricityPriceValues() *ElectricityPriceValues {
	return &ElectricityPriceValues{
		eventTime: time.Now(),
	}
}

func (epv *ElectricityPriceValues) EventTime() time.Time {
	return epv.eventTime
}

func (epv *ElectricityPriceValues) SetEnergyProviderName(energyProviderName string) *ElectricityPriceValues {
	epv.energyProviderName = energyProviderName
	return epv
}

func (epv *ElectricityPriceValues) EnergyProviderName() string {
	return epv.energyProviderName
}

func (epv *ElectricityPriceValues) SetConsumptionPrice(consumptionPrice float32) *ElectricityPriceValues {
	epv.consumptionPrice = consumptionPrice
	return epv
}

func (epv *ElectricityPriceValues) ConsumptionPrice() float32 {
	return epv.consumptionPrice
}

func (epv *ElectricityPriceValues) SetFeedbackPrice(feedbackPrice float32) *ElectricityPriceValues {
	epv.feedbackPrice = feedbackPrice
	return epv
}

func (epv *ElectricityPriceValues) FeedbackPrice() float32 {
	return epv.feedbackPrice
}

func (epv *ElectricityPriceValues) SetInterval(interval time.Duration) *ElectricityPriceValues {
	epv.interval = interval
	return epv
}

func (epv *ElectricityPriceValues) Interval() time.Duration {
	return epv.interval
}

func (epv *ElectricityPriceValues) SetPriceStartingTime(priceStartingTime time.Time) *ElectricityPriceValues {
	epv.priceStartingTime = priceStartingTime
	return epv
}

func (epv *ElectricityPriceValues) PriceStartingTime() time.Time {
	return epv.priceStartingTime
}

package domain

import (
	"time"
)

var ElectricityCosts = genericEventHandler[ElectricityCostsChangeListener, *ElectricityCostsValues]{
	listeners: make(map[ElectricityCostsChangeListener]func(values *ElectricityCostsValues) bool),
}

type ElectricityCostsChangeListener interface {
	HandleEvent(*ElectricityCostsValues)
}

type ElectricityCostsValues struct {
	eventTime              time.Time
	startTime              time.Time
	endTime                time.Time
	startConsumptionEnergy float64
	endConsumptionEnergy   float64
	startFeedbackEnergy    float64
	endFeedbackEnergy      float64
	energyProviderName     string
	consumptionPricePerKwh float32
	feedbackPricePerKwh    float32
}

func NewElectricityCostsValues() *ElectricityCostsValues {
	return &ElectricityCostsValues{
		eventTime: time.Now(),
	}
}

func (ecv *ElectricityCostsValues) EventTime() time.Time {
	return ecv.eventTime
}

func (ecv *ElectricityCostsValues) StartTime() time.Time {
	return ecv.startTime
}

func (ecv *ElectricityCostsValues) SetStartTime(startTime time.Time) *ElectricityCostsValues {
	ecv.startTime = startTime
	return ecv
}

func (ecv *ElectricityCostsValues) EndTime() time.Time {
	return ecv.endTime
}

func (ecv *ElectricityCostsValues) SetEndTime(endTime time.Time) *ElectricityCostsValues {
	ecv.endTime = endTime
	return ecv
}

func (ecv *ElectricityCostsValues) StartConsumptionEnergy() float64 {
	return ecv.startConsumptionEnergy
}

func (ecv *ElectricityCostsValues) SetStartConsumptionEnergy(startConsumptionEnergy float64) *ElectricityCostsValues {
	ecv.startConsumptionEnergy = startConsumptionEnergy
	return ecv
}

func (ecv *ElectricityCostsValues) EndConsumptionEnergy() float64 {
	return ecv.endConsumptionEnergy
}

func (ecv *ElectricityCostsValues) SetEndConsumptionEnergy(endConsumptionEnergy float64) *ElectricityCostsValues {
	ecv.endConsumptionEnergy = endConsumptionEnergy
	return ecv
}

func (ecv *ElectricityCostsValues) ConsumptionEnergy() float32 {
	return float32(ecv.EndConsumptionEnergy() - ecv.StartConsumptionEnergy())
}

func (ecv *ElectricityCostsValues) StartFeedbackEnergy() float64 {
	return ecv.startFeedbackEnergy
}

func (ecv *ElectricityCostsValues) SetStartFeedbackEnergy(startFeedbackEnergy float64) *ElectricityCostsValues {
	ecv.startFeedbackEnergy = startFeedbackEnergy
	return ecv
}

func (ecv *ElectricityCostsValues) EndFeedbackEnergy() float64 {
	return ecv.endFeedbackEnergy
}

func (ecv *ElectricityCostsValues) SetEndFeedbackEnergy(endFeedbackEnergy float64) *ElectricityCostsValues {
	ecv.endFeedbackEnergy = endFeedbackEnergy
	return ecv
}

func (ecv *ElectricityCostsValues) FeedbackEnergy() float32 {
	return float32(ecv.EndFeedbackEnergy() - ecv.StartFeedbackEnergy())
}

func (ecv *ElectricityCostsValues) EnergyProviderName() string {
	return ecv.energyProviderName
}

func (ecv *ElectricityCostsValues) SetEnergyProviderName(energyProviderName string) *ElectricityCostsValues {
	ecv.energyProviderName = energyProviderName
	return ecv
}

func (ecv *ElectricityCostsValues) ConsumptionPricePerKwh() float32 {
	return ecv.consumptionPricePerKwh
}

func (ecv *ElectricityCostsValues) SetConsumptionPricePerKwh(consumptionPricePerKwh float32) *ElectricityCostsValues {
	ecv.consumptionPricePerKwh = consumptionPricePerKwh
	return ecv
}

func (ecv *ElectricityCostsValues) FeedbackPricePerKwh() float32 {
	return ecv.feedbackPricePerKwh
}

func (ecv *ElectricityCostsValues) SetFeedbackPricePerKwh(feedbackPricePerKwh float32) *ElectricityCostsValues {
	ecv.feedbackPricePerKwh = feedbackPricePerKwh
	return ecv
}

func (ecv *ElectricityCostsValues) ConsumptionCosts() float32 {
	return ecv.ConsumptionPricePerKwh() * ecv.ConsumptionEnergy()
}

func (ecv *ElectricityCostsValues) FeedbackCosts() float32 {
	return ecv.FeedbackPricePerKwh() * ecv.FeedbackEnergy()
}

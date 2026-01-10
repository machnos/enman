package domain

import (
	"enman/internal/domain/events"
	"enman/internal/domain/repository"
	"time"
)

type PriceBasedElectricityConsumptionCalculator struct {
	repository                    repository.EnergyPrice
	providerName                  string
	batteries                     []*Battery
	peakDetectionStdDevMultiplier float32
	inChargeWindow                bool
	chargeWindowEndTime           time.Time
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

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) int {

	return 0
}

func (p *PriceBasedElectricityConsumptionCalculator) HandleEvent(values *events.ChargingPeriodValues) {
	p.inChargeWindow = values.PeriodType() == events.ChargingPeriodStart
	if p.inChargeWindow {
		p.chargeWindowEndTime = values.EndTime()
	} else {
		p.chargeWindowEndTime = time.Time{}
	}
}

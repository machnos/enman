package domain

import (
	"enman/internal/domain/events"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"math"
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

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) float32 {
	if !p.inChargeWindow || p.batteries == nil || len(p.batteries) == 0 {
		return 0
	}
	additions := float32(0)
	for _, battery := range p.batteries {
		additions += battery.NecessaryChargePower(100, p.chargeWindowEndTime.Sub(time.Now()))
	}
	return float32(math.Min(float64(availableChargePower), float64(additions)))
}

func (p *PriceBasedElectricityConsumptionCalculator) HandleEvent(values *events.ChargingPeriodValues) {
	p.inChargeWindow = values.PeriodType() == events.ChargingPeriodStart
	if p.inChargeWindow {
		log.Info("Entering price based charging window")
		p.chargeWindowEndTime = values.EndTime()
	} else {
		log.Info("Leaving price based charging window")
		p.chargeWindowEndTime = time.Time{}
	}
}

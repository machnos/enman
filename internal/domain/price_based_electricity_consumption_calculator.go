package domain

import (
	"enman/internal/domain/repository"
)

type PriceBasedElectricityConsumptionCalculator struct {
	repository                    repository.EnergyPrice
	providerName                  string
	batteries                     []*Battery
	peakDetectionStdDevMultiplier float32
}

// NewPriceBasedElectricityConsumptionCalculator creates a new calculator for determining
// battery charging power based on energy prices.
// peakDetectionStdDevMultiplier controls peak detection sensitivity (default 1.0):
//
//	0.5 = conservative (detects more peaks)
//	1.0 = optimal (balanced)
//	1.5 = aggressive (only extreme peaks)
//
// socThreshold is the SoC percentage below which survival charging is used (default 25.0)
func NewPriceBasedElectricityConsumptionCalculator(
	repository repository.EnergyPrice,
	providerName string,
	batteries []*Battery,
	peakDetectionStdDevMultiplier float32,
) *PriceBasedElectricityConsumptionCalculator {
	if peakDetectionStdDevMultiplier <= 0 {
		peakDetectionStdDevMultiplier = 1.0
	}
	return &PriceBasedElectricityConsumptionCalculator{
		repository:                    repository,
		providerName:                  providerName,
		batteries:                     batteries,
		peakDetectionStdDevMultiplier: peakDetectionStdDevMultiplier,
	}
}

func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) int {
	return 0
}

package domain

import (
	"context"
)

// PriceBasedElectricityConsumptionCalculator is a stub that will be replaced
// by BatteryScheduleListener once the MILP optimizer is implemented.
// For now, it provides the interface expected by GridTargetConsumptionCalculator.
type PriceBasedElectricityConsumptionCalculator struct {
}

func NewPriceBasedElectricityConsumptionCalculator() *PriceBasedElectricityConsumptionCalculator {
	return &PriceBasedElectricityConsumptionCalculator{}
}

// Start begins the background optimization process
func (p *PriceBasedElectricityConsumptionCalculator) Start(ctx context.Context) {
	// Stub: will be implemented as part of BatteryScheduleOptimizer integration
}

// Stop stops the background optimization goroutine
func (p *PriceBasedElectricityConsumptionCalculator) Stop() {
	// Stub: will be implemented as part of BatteryScheduleOptimizer integration
}

// CalculateAddition returns the additional power to draw from/feed to grid
// based on the current battery schedule.
// Positive = draw from grid (charge battery), Negative = feed to grid (discharge battery)
func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) float32 {
	// Stub: will be replaced by BatteryScheduleListener that listens to schedule events
	return 0
}

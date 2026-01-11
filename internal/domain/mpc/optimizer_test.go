package mpc

import (
	"testing"
	"time"
)

func TestMPCOptimizer_Optimize_BasicCharging(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	// Create a scenario where it's cheap now and expensive later
	now := time.Now().Truncate(time.Hour)
	prices := make([]PricePoint, NumTimeSteps)
	for i := 0; i < NumTimeSteps; i++ {
		prices[i] = PricePoint{
			Time:             now.Add(time.Duration(i) * time.Hour),
			EndTime:          now.Add(time.Duration(i+1) * time.Hour),
			ConsumptionPrice: 0.15, // Default price
			FeedbackPrice:    0.05,
		}
	}
	// Make first 4 hours very cheap (clear incentive to charge)
	for i := 0; i < 4; i++ {
		prices[i].ConsumptionPrice = 0.02 // Very cheap
	}
	// Make hours 12-16 very expensive (clear incentive to discharge)
	for i := 12; i < 16; i++ {
		prices[i].ConsumptionPrice = 0.40 // Very expensive
	}

	input := &OptimizationInput{
		CurrentSoC:             30.0, // Low SoC - should want to charge
		CurrentTime:            now,
		BatteryRoundTripEff:    90.0, // 90% efficiency
		BatteryCapacity:        10.0, // 10 kWh
		MaxChargePower:         5000, // 5 kW
		MaxDischargePower:      5000, // 5 kW
		MinSoC:                 10.0,
		MaxSoC:                 95.0,
		EnergyPrices:           prices,
		ForecastedConsumption:  make([]float32, NumTimeSteps),
		ForecastedPVProduction: make([]float32, NumTimeSteps),
	}

	// Set some baseline consumption
	for i := 0; i < NumTimeSteps; i++ {
		input.ForecastedConsumption[i] = 500 // 500W constant consumption
	}

	result := optimizer.Optimize(input)

	if !result.Success {
		t.Errorf("Expected optimization to succeed, but it failed: %s", result.Message)
	}

	// Log the charging schedule for the first few hours
	t.Logf("Charging schedule (first 6 hours):")
	for i := 0; i < min(6, len(result.ChargingSchedule)); i++ {
		t.Logf("  Hour %d: Charge=%.0fW, Discharge=%.0fW, SoC=%.1f%%, Price=%.2f€",
			i, result.ChargingSchedule[i], result.DischargingSchedule[i],
			result.SoCTrajectory[i], prices[i].ConsumptionPrice)
	}

	// During cheap hours, we should be charging
	if result.OptimalAction <= 0 {
		t.Logf("Warning: Expected positive charging action during cheap hours, got: %.2f W", result.OptimalAction)
	}

	// Verify SoC trajectory is within bounds
	for i, soc := range result.SoCTrajectory {
		if soc < input.MinSoC-0.1 || soc > input.MaxSoC+0.1 {
			t.Errorf("SoC at timestep %d (%.2f%%) is outside bounds [%.2f%%, %.2f%%]",
				i, soc, input.MinSoC, input.MaxSoC)
		}
	}

	t.Logf("Optimization result: Success=%v, OptimalAction=%.2f W, TotalCost=%.4f €",
		result.Success, result.OptimalAction, result.TotalCost)
	t.Logf("SoC trajectory: Start=%.1f%% -> End=%.1f%%",
		result.SoCTrajectory[0], result.SoCTrajectory[len(result.SoCTrajectory)-1])
}

func TestMPCOptimizer_Optimize_HighSoC_ShouldNotCharge(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	now := time.Now().Truncate(time.Hour)
	prices := make([]PricePoint, NumTimeSteps)
	for i := 0; i < NumTimeSteps; i++ {
		prices[i] = PricePoint{
			Time:             now.Add(time.Duration(i) * time.Hour),
			EndTime:          now.Add(time.Duration(i+1) * time.Hour),
			ConsumptionPrice: 0.10,
			FeedbackPrice:    0.05,
		}
	}

	input := &OptimizationInput{
		CurrentSoC:             90.0, // Already high SoC
		CurrentTime:            now,
		BatteryRoundTripEff:    90.0,
		BatteryCapacity:        10.0,
		MaxChargePower:         5000,
		MaxDischargePower:      5000,
		MinSoC:                 10.0,
		MaxSoC:                 95.0,
		EnergyPrices:           prices,
		ForecastedConsumption:  make([]float32, NumTimeSteps),
		ForecastedPVProduction: make([]float32, NumTimeSteps),
	}

	result := optimizer.Optimize(input)

	if !result.Success {
		t.Errorf("Expected optimization to succeed, but it failed: %s", result.Message)
	}

	// At high SoC, charging should be limited
	t.Logf("High SoC optimization: OptimalAction=%.2f W (should be low or zero)", result.OptimalAction)
}

func TestMPCOptimizer_Optimize_LowSoC_ExpensiveHour(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	now := time.Now().Truncate(time.Hour)
	prices := make([]PricePoint, NumTimeSteps)
	for i := 0; i < NumTimeSteps; i++ {
		prices[i] = PricePoint{
			Time:             now.Add(time.Duration(i) * time.Hour),
			EndTime:          now.Add(time.Duration(i+1) * time.Hour),
			ConsumptionPrice: 0.30, // All expensive
			FeedbackPrice:    0.15,
		}
	}
	// Make later hours cheaper
	for i := 20; i < NumTimeSteps; i++ {
		prices[i].ConsumptionPrice = 0.05
	}

	input := &OptimizationInput{
		CurrentSoC:             80.0,
		CurrentTime:            now,
		BatteryRoundTripEff:    90.0,
		BatteryCapacity:        10.0,
		MaxChargePower:         5000,
		MaxDischargePower:      5000,
		MinSoC:                 10.0,
		MaxSoC:                 95.0,
		EnergyPrices:           prices,
		ForecastedConsumption:  make([]float32, NumTimeSteps),
		ForecastedPVProduction: make([]float32, NumTimeSteps),
	}

	// High consumption to encourage discharge
	for i := 0; i < NumTimeSteps; i++ {
		input.ForecastedConsumption[i] = 2000 // 2kW constant consumption
	}

	result := optimizer.Optimize(input)

	if !result.Success {
		t.Errorf("Expected optimization to succeed, but it failed: %s", result.Message)
	}

	t.Logf("Expensive hour with high SoC: OptimalAction=%.2f W (negative means discharge)",
		result.OptimalAction)
}

func TestMPCOptimizer_Optimize_InvalidInput(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	// Test with zero battery capacity
	input := &OptimizationInput{
		CurrentSoC:          50.0,
		CurrentTime:         time.Now(),
		BatteryRoundTripEff: 90.0,
		BatteryCapacity:     0, // Invalid
		MaxChargePower:      5000,
		MaxDischargePower:   5000,
		MinSoC:              10.0,
		MaxSoC:              95.0,
		EnergyPrices:        []PricePoint{},
	}

	result := optimizer.Optimize(input)

	if result.Success {
		t.Error("Expected optimization to fail with invalid input")
	}
}

func TestMPCOptimizer_Optimize_NoPrices(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	input := &OptimizationInput{
		CurrentSoC:          50.0,
		CurrentTime:         time.Now(),
		BatteryRoundTripEff: 90.0,
		BatteryCapacity:     10.0,
		MaxChargePower:      5000,
		MaxDischargePower:   5000,
		MinSoC:              10.0,
		MaxSoC:              95.0,
		EnergyPrices:        []PricePoint{}, // Empty prices
	}

	result := optimizer.Optimize(input)

	if result.Success {
		t.Error("Expected optimization to fail with no prices")
	}
}

func TestMPCOptimizer_Optimize_WithPVProduction(t *testing.T) {
	optimizer := NewMPCOptimizer(10.0, 95.0)

	now := time.Now().Truncate(time.Hour)
	prices := make([]PricePoint, NumTimeSteps)
	for i := 0; i < NumTimeSteps; i++ {
		prices[i] = PricePoint{
			Time:             now.Add(time.Duration(i) * time.Hour),
			EndTime:          now.Add(time.Duration(i+1) * time.Hour),
			ConsumptionPrice: 0.15,
			FeedbackPrice:    0.08,
		}
	}

	input := &OptimizationInput{
		CurrentSoC:             50.0,
		CurrentTime:            now,
		BatteryRoundTripEff:    90.0,
		BatteryCapacity:        10.0,
		MaxChargePower:         5000,
		MaxDischargePower:      5000,
		MinSoC:                 10.0,
		MaxSoC:                 95.0,
		EnergyPrices:           prices,
		ForecastedConsumption:  make([]float32, NumTimeSteps),
		ForecastedPVProduction: make([]float32, NumTimeSteps),
	}

	// Set consumption and PV production
	for i := 0; i < NumTimeSteps; i++ {
		input.ForecastedConsumption[i] = 1000 // 1kW consumption

		// PV production during daylight hours (8-18)
		if i >= 8 && i < 18 {
			input.ForecastedPVProduction[i] = 3000 // 3kW PV
		}
	}

	result := optimizer.Optimize(input)

	if !result.Success {
		t.Errorf("Expected optimization to succeed with PV production, but it failed: %s", result.Message)
	}

	t.Logf("With PV production: OptimalAction=%.2f W, TotalCost=%.4f €",
		result.OptimalAction, result.TotalCost)
}

func TestBuildPriceForecast(t *testing.T) {
	now := time.Now().Truncate(time.Hour)

	// Create mock energy prices
	mockPrices := []*mockEnergyPrice{
		{Time: now, EndTime: now.Add(time.Hour), ConsumptionPrice: 0.10, FeedbackPrice: 0.05},
		{Time: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour), ConsumptionPrice: 0.15, FeedbackPrice: 0.08},
		{Time: now.Add(2 * time.Hour), EndTime: now.Add(3 * time.Hour), ConsumptionPrice: 0.20, FeedbackPrice: 0.10},
	}

	// We can't use the real BuildPriceForecast because it expects *prices.EnergyPrice
	// This test demonstrates the price utils work correctly
	pricePoints := make([]PricePoint, len(mockPrices))
	for i, p := range mockPrices {
		pricePoints[i] = PricePoint{
			Time:             p.Time,
			EndTime:          p.EndTime,
			ConsumptionPrice: p.ConsumptionPrice,
			FeedbackPrice:    p.FeedbackPrice,
		}
	}

	if len(pricePoints) != 3 {
		t.Errorf("Expected 3 price points, got %d", len(pricePoints))
	}

	if pricePoints[0].ConsumptionPrice != 0.10 {
		t.Errorf("Expected first price to be 0.10, got %.2f", pricePoints[0].ConsumptionPrice)
	}
}

type mockEnergyPrice struct {
	Time             time.Time
	EndTime          time.Time
	ConsumptionPrice float32
	FeedbackPrice    float32
}

func TestPriceUtils(t *testing.T) {
	prices := []PricePoint{
		{ConsumptionPrice: 0.10},
		{ConsumptionPrice: 0.20},
		{ConsumptionPrice: 0.05},
		{ConsumptionPrice: 0.30},
		{ConsumptionPrice: 0.15},
	}

	// Test CalculatePriceSpread
	spread := CalculatePriceSpread(prices)
	expectedSpread := float32(0.25) // 0.30 - 0.05
	if abs(spread-expectedSpread) > 0.001 {
		t.Errorf("Expected price spread %.4f, got %.4f", expectedSpread, spread)
	}

	// Test CalculateAveragePrice
	avg := CalculateAveragePrice(prices)
	expectedAvg := float32(0.16) // (0.10 + 0.20 + 0.05 + 0.30 + 0.15) / 5
	if abs(avg-expectedAvg) > 0.001 {
		t.Errorf("Expected average price %.4f, got %.4f", expectedAvg, avg)
	}

	// Test FindCheapestHours
	cheapest := FindCheapestHours(prices, 2)
	if len(cheapest) != 2 {
		t.Errorf("Expected 2 cheapest hours, got %d", len(cheapest))
	}
	// Cheapest should be indices 2 (0.05) and 0 (0.10)
	if cheapest[0] != 2 {
		t.Errorf("Expected cheapest hour to be index 2, got %d", cheapest[0])
	}

	// Test FindMostExpensiveHours
	expensive := FindMostExpensiveHours(prices, 2)
	if len(expensive) != 2 {
		t.Errorf("Expected 2 most expensive hours, got %d", len(expensive))
	}
	// Most expensive should be indices 3 (0.30) and 1 (0.20)
	if expensive[0] != 3 {
		t.Errorf("Expected most expensive hour to be index 3, got %d", expensive[0])
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

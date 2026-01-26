package optimizer

import (
	"enman/internal/domain"
	"enman/internal/domain/constants"
	"enman/internal/domain/prices"
	"testing"
	"time"
)

func TestNewBatteryScheduleOptimizer(t *testing.T) {
	// Create a system with batteries, PVs, and AC loads
	system := domain.NewSystem(time.UTC)
	system.SetGrid("main_grid", 230, 25, 3, 0, nil, nil)
	system.AddPv("pv1", nil, nil)
	system.AddPv("pv2", nil, nil)
	system.AddBattery(domain.NewBattery("battery1", 100, 1, 1, 51.2, nil))        // ~5.12 kWh
	system.AddBattery(domain.NewBattery("battery2", 50, 1, 1, 51.2, nil))         // ~2.56 kWh
	system.AddAcLoad("ev_charger", constants.EnergySourceRoleEvCharger, 100, nil) // factor=0
	system.AddAcLoad("heat_pump", constants.EnergySourceRoleUndefined, 50, nil)   // factor=0.5
	system.AddAcLoad("lights", constants.EnergySourceRoleUndefined, 0, nil)       // factor=1

	optimizer := NewBatteryScheduleOptimizer(system, nil, 0.85, 15)
	config := optimizer.Config()

	// Check energy provider (should be same as grid name)
	if config.EnergyProviderName != "main_grid" {
		t.Errorf("Expected EnergyProviderName 'main_grid', got '%s'", config.EnergyProviderName)
	}

	// Check battery names
	if len(config.BatteryNames) != 2 {
		t.Errorf("Expected 2 battery names, got %d", len(config.BatteryNames))
	}

	// Check PV names
	if len(config.PvNames) != 2 {
		t.Errorf("Expected 2 PV names, got %d", len(config.PvNames))
	}

	// Check AC load factors
	if len(config.AcLoadFactors) != 3 {
		t.Errorf("Expected 3 AC load factors, got %d", len(config.AcLoadFactors))
	}

	// Verify factors
	for _, alf := range config.AcLoadFactors {
		if alf.Name == "ev_charger" && alf.Factor != 0.0 {
			t.Errorf("Expected ev_charger factor 0.0, got %v", alf.Factor)
		}
		if alf.Name == "heat_pump" && alf.Factor != 0.5 {
			t.Errorf("Expected heat_pump factor 0.5, got %v", alf.Factor)
		}
		if alf.Name == "lights" && alf.Factor != 1.0 {
			t.Errorf("Expected lights factor 1.0, got %v", alf.Factor)
		}
	}

	// Check SoC limits
	if config.MinSoC != 15 {
		t.Errorf("Expected MinSoC 15, got %v", config.MinSoC)
	}
	if config.MaxSoC != 100 {
		t.Errorf("Expected MaxSoC 100, got %v", config.MaxSoC)
	}

	// Check efficiency
	if config.RoundTripEfficiency != 0.85 {
		t.Errorf("Expected RoundTripEfficiency 0.85, got %v", config.RoundTripEfficiency)
	}
}

func TestOptimizerOptimize(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)

	// Create test energy prices - cheap at night, expensive during day
	now := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)
	energyPrices := []*prices.EnergyPrice{
		{Time: now, EndTime: now.Add(time.Hour), ConsumptionPrice: 0.05},
		{Time: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour), ConsumptionPrice: 0.05},
		{Time: now.Add(2 * time.Hour), EndTime: now.Add(3 * time.Hour), ConsumptionPrice: 0.06},
		{Time: now.Add(3 * time.Hour), EndTime: now.Add(4 * time.Hour), ConsumptionPrice: 0.08},
		{Time: now.Add(4 * time.Hour), EndTime: now.Add(5 * time.Hour), ConsumptionPrice: 0.10},
		{Time: now.Add(5 * time.Hour), EndTime: now.Add(6 * time.Hour), ConsumptionPrice: 0.15},
		{Time: now.Add(6 * time.Hour), EndTime: now.Add(7 * time.Hour), ConsumptionPrice: 0.25},
		{Time: now.Add(7 * time.Hour), EndTime: now.Add(8 * time.Hour), ConsumptionPrice: 0.30},
		{Time: now.Add(8 * time.Hour), EndTime: now.Add(9 * time.Hour), ConsumptionPrice: 0.28},
		{Time: now.Add(9 * time.Hour), EndTime: now.Add(10 * time.Hour), ConsumptionPrice: 0.20},
		{Time: now.Add(10 * time.Hour), EndTime: now.Add(11 * time.Hour), ConsumptionPrice: 0.15},
		{Time: now.Add(11 * time.Hour), EndTime: now.Add(12 * time.Hour), ConsumptionPrice: 0.10},
	}

	slots := optimizer.OptimizeWithPrices(now, energyPrices)

	if len(slots) == 0 {
		t.Fatal("Expected slots to be generated")
	}

	// Verify slots are in chronological order
	for i := 1; i < len(slots); i++ {
		if slots[i].StartTime().Before(slots[i-1].StartTime()) {
			t.Errorf("Slots not in chronological order at index %d", i)
		}
	}

	// Verify that cheap slots have positive charge power (charging)
	cheapSlotFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() <= 0.06 && slot.ChargePower() > 0 {
			cheapSlotFound = true
			break
		}
	}
	if !cheapSlotFound {
		t.Error("Expected at least one cheap slot to be charging")
	}

	// Verify that expensive slots have negative charge power (discharging)
	expensiveSlotFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() >= 0.25 && slot.ChargePower() < 0 {
			expensiveSlotFound = true
			break
		}
	}
	if !expensiveSlotFound {
		t.Error("Expected at least one expensive slot to be discharging")
	}

	// Verify predicted SoC stays within bounds
	for _, slot := range slots {
		if slot.PredictedSoC() < config.MinSoC {
			t.Errorf("Predicted SoC %v below minimum %v at slot %v", slot.PredictedSoC(), config.MinSoC, slot.StartTime())
		}
		if slot.PredictedSoC() > config.MaxSoC {
			t.Errorf("Predicted SoC %v above maximum %v at slot %v", slot.PredictedSoC(), config.MaxSoC, slot.StartTime())
		}
	}
}

func TestOptimizerWithPvExcess(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC)

	// Create predictions with excess PV during midday
	predictions := []*slotPrediction{
		{
			startTime:             now,
			endTime:               now.Add(time.Hour),
			price:                 0.15,
			predictedPvPowerW:     3000,  // High PV
			predictedConsumptionW: 1000,  // Low consumption
			netGridDemandW:        -2000, // Excess PV
		},
		{
			startTime:             now.Add(time.Hour),
			endTime:               now.Add(2 * time.Hour),
			price:                 0.15,
			predictedPvPowerW:     4000,
			predictedConsumptionW: 1000,
			netGridDemandW:        -3000, // More excess PV
		},
		{
			startTime:             now.Add(2 * time.Hour),
			endTime:               now.Add(3 * time.Hour),
			price:                 0.25, // Expensive evening
			predictedPvPowerW:     0,    // No PV
			predictedConsumptionW: 2000, // High consumption
			netGridDemandW:        2000, // Need from grid
		},
	}

	slots := optimizer.Optimize(50, predictions)

	if len(slots) != 3 {
		t.Fatalf("Expected 3 slots, got %d", len(slots))
	}

	// First two slots should be charging (storing excess PV)
	if slots[0].ChargePower() <= 0 {
		t.Error("Expected first slot to be charging from excess PV")
	}
	if slots[1].ChargePower() <= 0 {
		t.Error("Expected second slot to be charging from excess PV")
	}

	// Third slot should be discharging (covering expensive consumption)
	if slots[2].ChargePower() >= 0 {
		t.Error("Expected third slot to be discharging to cover consumption")
	}
}

func TestOptimizerWithHighConsumption(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	// Cheap night with low consumption, expensive day with high consumption
	predictions := []*slotPrediction{
		{
			startTime:             now,
			endTime:               now.Add(time.Hour),
			price:                 0.05, // Cheap
			predictedPvPowerW:     0,
			predictedConsumptionW: 500, // Low night consumption
			netGridDemandW:        500,
		},
		{
			startTime:             now.Add(6 * time.Hour),
			endTime:               now.Add(7 * time.Hour),
			price:                 0.30, // Expensive
			predictedPvPowerW:     0,
			predictedConsumptionW: 4000, // High morning consumption
			netGridDemandW:        4000,
		},
	}

	slots := optimizer.Optimize(50, predictions)

	if len(slots) != 2 {
		t.Fatalf("Expected 2 slots, got %d", len(slots))
	}

	// First slot should charge (cheap period)
	if slots[0].ChargePower() <= 0 {
		t.Error("Expected first slot to be charging during cheap period")
	}

	// Second slot should discharge (expensive + high consumption)
	if slots[1].ChargePower() >= 0 {
		t.Error("Expected second slot to be discharging during expensive high-consumption period")
	}
}

func TestOptimizerWithNoArbitrageOpportunity(t *testing.T) {
	config := DefaultConfig()
	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)

	// Create flat prices - no arbitrage opportunity
	now := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)
	energyPrices := []*prices.EnergyPrice{
		{Time: now, EndTime: now.Add(time.Hour), ConsumptionPrice: 0.15},
		{Time: now.Add(time.Hour), EndTime: now.Add(2 * time.Hour), ConsumptionPrice: 0.15},
		{Time: now.Add(2 * time.Hour), EndTime: now.Add(3 * time.Hour), ConsumptionPrice: 0.15},
		{Time: now.Add(3 * time.Hour), EndTime: now.Add(4 * time.Hour), ConsumptionPrice: 0.15},
	}

	slots := optimizer.OptimizeWithPrices(now, energyPrices)

	// With flat prices and no PV/consumption data, most slots should be idle
	idleCount := 0
	for _, slot := range slots {
		if slot.IsIdle() {
			idleCount++
		}
	}

	if idleCount < len(slots)/2 {
		t.Errorf("Expected most slots to be idle with flat prices, got %d idle out of %d", idleCount, len(slots))
	}
}

func TestOptimizerRespectsMinSoC(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      20,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	// High price with high consumption - would want to discharge a lot
	predictions := []*slotPrediction{
		{startTime: now, endTime: now.Add(time.Hour), price: 0.50, netGridDemandW: 5000},
		{startTime: now.Add(time.Hour), endTime: now.Add(2 * time.Hour), price: 0.50, netGridDemandW: 5000},
		{startTime: now.Add(2 * time.Hour), endTime: now.Add(3 * time.Hour), price: 0.50, netGridDemandW: 5000},
	}

	// Start with low SoC close to minimum
	slots := optimizer.Optimize(25, predictions)

	for _, slot := range slots {
		if slot.PredictedSoC() < config.MinSoC {
			t.Errorf("SoC dropped below minimum: got %v, minimum is %v", slot.PredictedSoC(), config.MinSoC)
		}
	}
}

func TestOptimizerRespectsMaxSoC(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      90,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 0, 0, 0, 0, time.UTC)

	// Low price with excess PV - would want to charge a lot
	predictions := []*slotPrediction{
		{startTime: now, endTime: now.Add(time.Hour), price: 0.01, netGridDemandW: -5000},
		{startTime: now.Add(time.Hour), endTime: now.Add(2 * time.Hour), price: 0.01, netGridDemandW: -5000},
		{startTime: now.Add(2 * time.Hour), endTime: now.Add(3 * time.Hour), price: 0.01, netGridDemandW: -5000},
	}

	// Start with high SoC close to maximum
	slots := optimizer.Optimize(85, predictions)

	for _, slot := range slots {
		if slot.PredictedSoC() > config.MaxSoC {
			t.Errorf("SoC exceeded maximum: got %v, maximum is %v", slot.PredictedSoC(), config.MaxSoC)
		}
	}
}

func TestOptimizerChargesDuringCheapNightHours(t *testing.T) {
	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          15.0, // ~304Ah * 51.2V / 1000
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 23, 0, 0, 0, time.UTC)

	// Simulate realistic price pattern: cheap at night, expensive during day
	// Based on actual production data
	predictions := []*slotPrediction{
		// Night hours - cheap prices, no PV
		{startTime: now, endTime: now.Add(15 * time.Minute), price: 0.267, netGridDemandW: 500},
		{startTime: now.Add(15 * time.Minute), endTime: now.Add(30 * time.Minute), price: 0.262, netGridDemandW: 500},
		{startTime: now.Add(30 * time.Minute), endTime: now.Add(45 * time.Minute), price: 0.260, netGridDemandW: 500},
		{startTime: now.Add(45 * time.Minute), endTime: now.Add(60 * time.Minute), price: 0.259, netGridDemandW: 500},
		// More night hours
		{startTime: now.Add(1 * time.Hour), endTime: now.Add(1*time.Hour + 15*time.Minute), price: 0.258, netGridDemandW: 500},
		{startTime: now.Add(1*time.Hour + 15*time.Minute), endTime: now.Add(1*time.Hour + 30*time.Minute), price: 0.256, netGridDemandW: 500},
		{startTime: now.Add(1*time.Hour + 30*time.Minute), endTime: now.Add(1*time.Hour + 45*time.Minute), price: 0.253, netGridDemandW: 500},
		{startTime: now.Add(1*time.Hour + 45*time.Minute), endTime: now.Add(2 * time.Hour), price: 0.251, netGridDemandW: 500},
		// Cheapest hours around 5-6 AM
		{startTime: now.Add(6 * time.Hour), endTime: now.Add(6*time.Hour + 15*time.Minute), price: 0.249, netGridDemandW: 500},
		{startTime: now.Add(6*time.Hour + 15*time.Minute), endTime: now.Add(6*time.Hour + 30*time.Minute), price: 0.250, netGridDemandW: 500},
		// Morning - prices rising
		{startTime: now.Add(8 * time.Hour), endTime: now.Add(8*time.Hour + 15*time.Minute), price: 0.310, netGridDemandW: 800},
		{startTime: now.Add(8*time.Hour + 15*time.Minute), endTime: now.Add(8*time.Hour + 30*time.Minute), price: 0.335, netGridDemandW: 800},
		// Midday - expensive with PV
		{startTime: now.Add(12 * time.Hour), endTime: now.Add(12*time.Hour + 15*time.Minute), price: 0.340, netGridDemandW: -1000},                // Excess PV
		{startTime: now.Add(12*time.Hour + 15*time.Minute), endTime: now.Add(12*time.Hour + 30*time.Minute), price: 0.350, netGridDemandW: -1500}, // More excess PV
		// Peak evening - most expensive
		{startTime: now.Add(18 * time.Hour), endTime: now.Add(18*time.Hour + 15*time.Minute), price: 0.360, netGridDemandW: 1500},
		{startTime: now.Add(18*time.Hour + 15*time.Minute), endTime: now.Add(18*time.Hour + 30*time.Minute), price: 0.378, netGridDemandW: 1500},
	}

	currentSoC := float32(50)
	slots := optimizer.Optimize(currentSoC, predictions)

	// Debug output
	for _, slot := range slots {
		t.Logf("Slot %v: price=%.4f, chargePower=%.1f, SoC=%.1f",
			slot.StartTime().Format("15:04"), slot.PricePerKwh(), slot.ChargePower(), slot.PredictedSoC())
	}

	// Verify that at least some cheap night slots (price < 0.27) are charging
	nightChargingFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() < 0.27 && slot.ChargePower() > 0 {
			nightChargingFound = true
			t.Logf("Found night charging: %v at price %.4f with power %.1fW",
				slot.StartTime().Format("15:04"), slot.PricePerKwh(), slot.ChargePower())
		}
	}

	if !nightChargingFound {
		t.Error("Expected at least one cheap night slot to be charging from grid")
	}

	// Verify PV slots are still being stored
	pvChargingFound := false
	for _, slot := range slots {
		// PV slots have negative netGridDemandW in predictions and positive chargePower in output
		if slot.PricePerKwh() > 0.33 && slot.ChargePower() > 0 {
			pvChargingFound = true
		}
	}
	if !pvChargingFound {
		t.Error("Expected excess PV to be stored in battery")
	}

	// Verify peak slots are discharging
	dischargingFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() >= 0.36 && slot.ChargePower() < 0 {
			dischargingFound = true
		}
	}
	if !dischargingFound {
		t.Error("Expected peak price slots to be discharging")
	}
}

func TestOptimizerSurvivalCharging(t *testing.T) {
	// Test scenario: Battery would hit MinSoC before reaching the cheapest charging slots
	// The optimizer should add "survival charging" at moderately cheap slots to survive
	// until the cheapest slots arrive.
	//
	// Timeline:
	// - Slots 0-3: Moderate price (0.28-0.30), high consumption - battery draining
	// - Slots 4-6: Peak price (0.38-0.40) - want to discharge but need energy
	// - Slots 7-10: Cheapest price (0.20-0.22) - optimal charging time

	config := &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}

	optimizer := NewBatteryScheduleOptimizerWithConfig(config, nil)
	now := time.Date(2026, 1, 25, 18, 0, 0, 0, time.UTC)

	predictions := []*slotPrediction{
		// Slots 0-3: Moderate price with consumption (battery draining)
		{startTime: now, endTime: now.Add(15 * time.Minute), price: 0.28, netGridDemandW: 2000},
		{startTime: now.Add(15 * time.Minute), endTime: now.Add(30 * time.Minute), price: 0.29, netGridDemandW: 2000},
		{startTime: now.Add(30 * time.Minute), endTime: now.Add(45 * time.Minute), price: 0.30, netGridDemandW: 2000},
		{startTime: now.Add(45 * time.Minute), endTime: now.Add(60 * time.Minute), price: 0.29, netGridDemandW: 2000},
		// Slots 4-6: Peak prices - want to discharge here!
		{startTime: now.Add(1 * time.Hour), endTime: now.Add(1*time.Hour + 15*time.Minute), price: 0.38, netGridDemandW: 1500},
		{startTime: now.Add(1*time.Hour + 15*time.Minute), endTime: now.Add(1*time.Hour + 30*time.Minute), price: 0.40, netGridDemandW: 1500},
		{startTime: now.Add(1*time.Hour + 30*time.Minute), endTime: now.Add(1*time.Hour + 45*time.Minute), price: 0.39, netGridDemandW: 1500},
		// Slots 7-10: Cheapest prices - optimal charging time
		{startTime: now.Add(2 * time.Hour), endTime: now.Add(2*time.Hour + 15*time.Minute), price: 0.20, netGridDemandW: 500},
		{startTime: now.Add(2*time.Hour + 15*time.Minute), endTime: now.Add(2*time.Hour + 30*time.Minute), price: 0.21, netGridDemandW: 500},
		{startTime: now.Add(2*time.Hour + 30*time.Minute), endTime: now.Add(2*time.Hour + 45*time.Minute), price: 0.22, netGridDemandW: 500},
		{startTime: now.Add(2*time.Hour + 45*time.Minute), endTime: now.Add(3 * time.Hour), price: 0.21, netGridDemandW: 500},
	}

	// Start with low SoC - would hit MinSoC before reaching cheap slots without survival charging
	currentSoC := float32(20)
	slots := optimizer.Optimize(currentSoC, predictions)

	// Debug output
	for i, slot := range slots {
		action := "idle"
		if slot.ChargePower() > 0 {
			action = "charging"
		} else if slot.ChargePower() < 0 {
			action = "discharging"
		}
		t.Logf("Slot %d (%v): price=%.2f, power=%.0f (%s), SoC=%.1f%%",
			i, slot.StartTime().Format("15:04"), slot.PricePerKwh(), slot.ChargePower(), action, slot.PredictedSoC())
	}

	// Verify SoC never drops below MinSoC
	for i, slot := range slots {
		if slot.PredictedSoC() < config.MinSoC {
			t.Errorf("Slot %d: SoC dropped below minimum: got %.1f%%, minimum is %.1f%%",
				i, slot.PredictedSoC(), config.MinSoC)
		}
	}

	// Verify that at least one peak slot (price >= 0.38) is discharging
	peakDischargeFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() >= 0.38 && slot.ChargePower() < 0 {
			peakDischargeFound = true
			break
		}
	}
	if !peakDischargeFound {
		t.Error("Expected at least one peak price slot to be discharging - survival charging should have preserved enough energy")
	}

	// Verify that the cheapest slots (price <= 0.22) are charging
	cheapChargingFound := false
	for _, slot := range slots {
		if slot.PricePerKwh() <= 0.22 && slot.ChargePower() > 0 {
			cheapChargingFound = true
			break
		}
	}
	if !cheapChargingFound {
		t.Error("Expected cheapest slots to be charging")
	}
}

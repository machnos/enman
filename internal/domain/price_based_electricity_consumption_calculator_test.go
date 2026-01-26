package domain

import (
	"enman/internal/domain/battery"
	"enman/internal/domain/events"
	"testing"
	"time"
)

func TestPriceBasedElectricityConsumptionCalculator_HandleEvent(t *testing.T) {
	calc := NewPriceBasedElectricityConsumptionCalculator()

	// Create a schedule slot for the current time
	now := time.Now()
	slot := battery.NewScheduleSlot(now.Add(-1*time.Minute), now.Add(14*time.Minute)).
		SetChargePower(3000).
		SetChargingSource(battery.ChargingSourceGrid)

	// Create and handle the event
	event := events.NewBatteryScheduleSlotEvent(slot)
	calc.HandleEvent(event)

	// Check that the slot was stored
	slots := calc.GetScheduledSlots()
	if len(slots) != 1 {
		t.Errorf("Expected 1 slot, got %d", len(slots))
	}

	// Check that we can get the current slot
	currentSlot := calc.GetCurrentSlot()
	if currentSlot == nil {
		t.Error("Expected current slot to be found")
	} else if currentSlot.ChargePower() != 3000 {
		t.Errorf("Expected charge power 3000, got %.0f", currentSlot.ChargePower())
	}
}

func TestPriceBasedElectricityConsumptionCalculator_CalculateAddition_Charging(t *testing.T) {
	calc := NewPriceBasedElectricityConsumptionCalculator()

	// Create a charging slot for the current time
	now := time.Now()
	slot := battery.NewScheduleSlot(now.Add(-1*time.Minute), now.Add(14*time.Minute)).
		SetChargePower(5000).
		SetChargingSource(battery.ChargingSourceGrid)

	event := events.NewBatteryScheduleSlotEvent(slot)
	calc.HandleEvent(event)

	// Calculate addition with enough available power
	addition := calc.CalculateAddition(10000)
	if addition != 5000 {
		t.Errorf("Expected addition 5000, got %.0f", addition)
	}

	// Calculate addition with limited available power
	addition = calc.CalculateAddition(3000)
	if addition != 3000 {
		t.Errorf("Expected addition 3000 (limited by available), got %.0f", addition)
	}
}

func TestPriceBasedElectricityConsumptionCalculator_CalculateAddition_Discharging(t *testing.T) {
	calc := NewPriceBasedElectricityConsumptionCalculator()

	// Create a discharging slot for the current time
	now := time.Now()
	slot := battery.NewScheduleSlot(now.Add(-1*time.Minute), now.Add(14*time.Minute)).
		SetChargePower(-2000). // Negative = discharging
		SetChargingSource(battery.ChargingSourceNone)

	event := events.NewBatteryScheduleSlotEvent(slot)
	calc.HandleEvent(event)

	// Discharge power should be returned as-is (negative)
	addition := calc.CalculateAddition(10000)
	if addition != -2000 {
		t.Errorf("Expected addition -2000 (discharging), got %.0f", addition)
	}
}

func TestPriceBasedElectricityConsumptionCalculator_CalculateAddition_NoActiveSlot(t *testing.T) {
	calc := NewPriceBasedElectricityConsumptionCalculator()

	// Create a slot for the future (not active yet)
	now := time.Now()
	slot := battery.NewScheduleSlot(now.Add(1*time.Hour), now.Add(2*time.Hour)).
		SetChargePower(5000).
		SetChargingSource(battery.ChargingSourceGrid)

	event := events.NewBatteryScheduleSlotEvent(slot)
	calc.HandleEvent(event)

	// Should return 0 when no active slot
	addition := calc.CalculateAddition(10000)
	if addition != 0 {
		t.Errorf("Expected addition 0 (no active slot), got %.0f", addition)
	}
}

func TestPriceBasedElectricityConsumptionCalculator_SlotReplacement(t *testing.T) {
	calc := NewPriceBasedElectricityConsumptionCalculator()

	// Create a slot
	now := time.Now()
	startTime := now.Add(-1 * time.Minute)
	endTime := now.Add(14 * time.Minute)

	slot1 := battery.NewScheduleSlot(startTime, endTime).
		SetChargePower(3000).
		SetChargingSource(battery.ChargingSourceGrid)

	calc.HandleEvent(events.NewBatteryScheduleSlotEvent(slot1))

	// Replace with a new slot with the same start time but different power
	slot2 := battery.NewScheduleSlot(startTime, endTime).
		SetChargePower(4500).
		SetChargingSource(battery.ChargingSourceGrid)

	calc.HandleEvent(events.NewBatteryScheduleSlotEvent(slot2))

	// Should still have only 1 slot
	slots := calc.GetScheduledSlots()
	if len(slots) != 1 {
		t.Errorf("Expected 1 slot (replaced), got %d", len(slots))
	}

	// Should use the new power value
	addition := calc.CalculateAddition(10000)
	if addition != 4500 {
		t.Errorf("Expected addition 4500 (replaced slot), got %.0f", addition)
	}
}

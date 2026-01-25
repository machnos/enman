package battery

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewScheduleSlot(t *testing.T) {
	start := time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 25, 11, 0, 0, 0, time.UTC)

	slot := NewScheduleSlot(start, end)

	if slot.StartTime() != start {
		t.Errorf("Expected start time %v, got %v", start, slot.StartTime())
	}
	if slot.EndTime() != end {
		t.Errorf("Expected end time %v, got %v", end, slot.EndTime())
	}
	if slot.Duration() != time.Hour {
		t.Errorf("Expected duration 1 hour, got %v", slot.Duration())
	}
}

func TestScheduleSlotChargingDischarging(t *testing.T) {
	start := time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 25, 11, 0, 0, 0, time.UTC)

	// Test charging slot (positive power)
	chargingSlot := NewScheduleSlot(start, end).SetChargePower(5000)
	if !chargingSlot.IsCharging() {
		t.Error("Expected slot to be charging")
	}
	if chargingSlot.IsDischarging() {
		t.Error("Expected slot to not be discharging")
	}
	if chargingSlot.ChargePower() != 5000 {
		t.Errorf("Expected charge power 5000, got %v", chargingSlot.ChargePower())
	}
	if chargingSlot.Energy() != 5.0 {
		t.Errorf("Expected 5 kWh energy, got %v", chargingSlot.Energy())
	}

	// Test discharging slot (negative power)
	dischargingSlot := NewScheduleSlot(start, end).SetChargePower(-3000)
	if dischargingSlot.IsCharging() {
		t.Error("Expected slot to not be charging")
	}
	if !dischargingSlot.IsDischarging() {
		t.Error("Expected slot to be discharging")
	}
	if dischargingSlot.ChargePower() != -3000 {
		t.Errorf("Expected charge power -3000, got %v", dischargingSlot.ChargePower())
	}
	if dischargingSlot.Energy() != -3.0 {
		t.Errorf("Expected -3 kWh energy, got %v", dischargingSlot.Energy())
	}

	// Test idle slot
	idleSlot := NewScheduleSlot(start, end)
	if !idleSlot.IsIdle() {
		t.Error("Expected slot to be idle")
	}
}

func TestScheduleSlotJSONSerialization(t *testing.T) {
	start := time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 25, 11, 0, 0, 0, time.UTC)

	original := NewScheduleSlot(start, end).
		SetChargePower(5000).
		SetPredictedSoC(85.5).
		SetPricePerKwh(0.15)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal slot: %v", err)
	}

	var restored ScheduleSlot
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal slot: %v", err)
	}

	if !restored.StartTime().Equal(original.StartTime()) {
		t.Errorf("StartTime mismatch: expected %v, got %v", original.StartTime(), restored.StartTime())
	}
	if !restored.EndTime().Equal(original.EndTime()) {
		t.Errorf("EndTime mismatch: expected %v, got %v", original.EndTime(), restored.EndTime())
	}
	if restored.ChargePower() != original.ChargePower() {
		t.Errorf("ChargePower mismatch: expected %v, got %v", original.ChargePower(), restored.ChargePower())
	}
	if restored.PredictedSoC() != original.PredictedSoC() {
		t.Errorf("PredictedSoC mismatch: expected %v, got %v", original.PredictedSoC(), restored.PredictedSoC())
	}
	if restored.PricePerKwh() != original.PricePerKwh() {
		t.Errorf("PricePerKwh mismatch: expected %v, got %v", original.PricePerKwh(), restored.PricePerKwh())
	}
}

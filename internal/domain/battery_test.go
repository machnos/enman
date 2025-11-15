package domain

import (
	"enman/internal/domain/battery"
	"testing"
	"time"
)

func TestBattery_MaxChargePower(t *testing.T) {
	capacity := uint16(300)
	voltage := float32(51.2)
	chargingCoefficient := float32(0.5)
	b := &Battery{
		capacity:            capacity,
		chargingCoefficient: chargingCoefficient,
		voltage:             voltage,
	}
	expectedChargePower := float32(capacity) * voltage * chargingCoefficient
	if b.MaxChargePower() != expectedChargePower {
		t.Errorf("Expected a charge power of %f, got %f", expectedChargePower, b.MaxChargePower())
	}
}

func TestBattery_MaxDischargePower(t *testing.T) {
	capacity := uint16(300)
	voltage := float32(51.2)
	dischargingCoefficient := float32(0.5)
	b := &Battery{
		capacity:               capacity,
		dischargingCoefficient: dischargingCoefficient,
		voltage:                voltage,
	}
	expectedDischargePower := float32(capacity) * voltage * dischargingCoefficient
	if b.MaxDischargePower() != expectedDischargePower {
		t.Errorf("Expected a discharge power of %f, got %f", expectedDischargePower, b.MaxDischargePower())
	}
}

func TestBattery_AvailableCapacity(t *testing.T) {
	state := battery.NewState()
	b := &Battery{
		capacity: 100,
		state:    state,
	}
	state.SetSoH(99)
	expected := float32(99)
	if b.AvailableCapacity() != expected {
		t.Errorf("Expected an available capacity of %f, got %f", expected, b.AvailableCapacity())
	}
}

func TestBattery_ChargeDuration(t *testing.T) {
	state := battery.NewState()
	state.SetSoH(75)
	state.SetSoC(50)
	b := &Battery{
		capacity:            100,
		voltage:             float32(51.2),
		state:               state,
		chargingCoefficient: float32(1),
	}
	// We have a 100Ah battery at 51.2v -> 5.12kWh
	// The SoH = 75% so 0.75 * 5.12 = 3.84kWh actual capacity
	// The SoC = 50% so we need to add 1.92kWh to the battery.

	// We charge with 1000 watts, so we expect the charge to be exactly 1,92 hours -> 6912 seconds
	duration := b.ChargeDuration(1000, 100)
	expectedDuration := time.Duration(6912) * time.Second
	if expectedDuration != duration {
		t.Errorf("Expected a charge duration of %v, got %v", expectedDuration, duration)
	}
}

func TestBattery_NecessaryChargePower(t *testing.T) {
	state := battery.NewState()
	state.SetSoH(75)
	state.SetSoC(50)
	b := &Battery{
		capacity:            100,
		voltage:             float32(51.2),
		state:               state,
		chargingCoefficient: float32(1),
	}
	// We have a 100Ah battery at 51.2v -> 5.12kWh
	// The SoH = 75% so 0.75 * 5.12 = 3.84kWh actual capacity
	// The SoC = 50% so we need to add 1.92kWh to the battery.

	// So, if we charge 1 hour, we expect 1920 watts of necessary charge power.
	power := b.NecessaryChargePower(100, time.Duration(1)*time.Hour)
	expectedPower := float32(1920)
	if expectedPower != power {
		t.Errorf("Expected a charge power of %v, got %v", expectedPower, power)
	}

}

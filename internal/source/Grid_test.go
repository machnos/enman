package source

import (
	"math/rand"
	"testing"
)

func TestLessThanMinVoltage(t *testing.T) {
	const invalidVoltage = MinVoltage - 1
	_, err := NewGrid(invalidVoltage, MinPhases, MinCurrentPerPhase)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with a voltage of %d "+
			"because it is less than the minimum of %d.", invalidVoltage, MinVoltage)
	}
}

func TestMoreThanMaxVoltage(t *testing.T) {
	const invalidVoltage = MaxVoltage + 1
	_, err := NewGrid(invalidVoltage, MinPhases, MinCurrentPerPhase)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with a voltage of %d "+
			"because it is more than the maximum of %d.", invalidVoltage, MaxVoltage)
	}
}

func TestVoltage(t *testing.T) {
	// Initialize a random voltage
	voltage := int16(rand.Intn(int(MaxVoltage)))
	if voltage == 0 {
		voltage += 1
	}
	grid, err := NewGrid(voltage, MinPhases, MinCurrentPerPhase)
	if err != nil {
		t.Error(err)
	}
	if grid == nil {
		t.Error("Expected a Grid pointer, got nil.")
	}
	if grid.Voltage() != voltage {
		t.Errorf("Grid voltage not equal. Expected %d got %d.", voltage, grid.Voltage())
	}
}

func TestLessThanMinPhases(t *testing.T) {
	const invalidPhases = MinPhases - 1
	_, err := NewGrid(MinVoltage, invalidPhases, MinCurrentPerPhase)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with %d phases "+
			"because it is less than the minimum of %d phases.", invalidPhases, MinPhases)
	}
}

func TestMinToMaxPhases(t *testing.T) {
	for phases := MinPhases; phases <= MaxPhases; phases++ {
		grid, err := NewGrid(MinVoltage, phases, MinCurrentPerPhase)
		if err != nil {
			t.Error(err)
		}
		if grid.Phases() != phases {
			t.Errorf("Grid phases not equal. Expected %d got %d.", phases, grid.Phases())
		}
	}
}

func TestMoreThanMaxPhases(t *testing.T) {
	const invalidPhases = MaxPhases + 1
	_, err := NewGrid(MinVoltage, invalidPhases, MinCurrentPerPhase)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with %d phases "+
			"because it is more than the maximum of %d phases.", invalidPhases, MaxPhases)
	}
}

func TestLessThanMinCurrentPerPhase(t *testing.T) {
	const invalidCurrent = MinCurrentPerPhase - 1
	_, err := NewGrid(MinVoltage, MinPhases, invalidCurrent)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with a max current of %f "+
			"because it is less than the minimum of %f.", invalidCurrent, MinCurrentPerPhase)
	}
}

func TestMoreThanMaxCurrentPerPhase(t *testing.T) {
	const invalidCurrent = MaxCurrentPerPhase + 1
	_, err := NewGrid(MinVoltage, MinPhases, invalidCurrent)
	if err == nil {
		t.Errorf("Expected error when initiating a grid with a max current of %f "+
			"because it is more than the maximum of %f.", invalidCurrent, MinCurrentPerPhase)
	}
}

func TestMaxCurrentPerPhase(t *testing.T) {
	// Initialize a random max current
	maxCurrent := rand.Float32()
	if maxCurrent == 0 {
		maxCurrent += 0.1
	}
	maxCurrent *= MaxCurrentPerPhase

	grid, err := NewGrid(MinVoltage, MinPhases, maxCurrent)
	if err != nil {
		t.Error(err)
	}
	if grid == nil {
		t.Error("Expected a Grid pointer, got nil.")
	}
	if grid.MaxCurrentPerPhase() != maxCurrent {
		t.Errorf("Grid maxCurrentPerPhase not equal. Expected %f got %f.", maxCurrent, grid.MaxCurrentPerPhase())
	}
}

func TestPowerPerPhase(t *testing.T) {
	expected := int32(230 * 25)
	grid, _ := NewGrid(230, 1, 25)
	if grid.MaxPowerPerPhase() != expected {
		t.Errorf("Grid MaxPowerPerPhase not equal. Expected %d got %d.", expected, grid.MaxPowerPerPhase())
	}
}

func TestMaxTotalPower(t *testing.T) {
	expected := int32(110 * 40 * 3)
	grid, _ := NewGrid(110, 3, 40)
	if grid.MaxTotalPower() != expected {
		t.Errorf("Grid MaxTotalPower not equal. Expected %d got %d.", expected, grid.MaxTotalPower())
	}
}

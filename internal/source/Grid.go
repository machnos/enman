package source

import (
	"fmt"
)

const (
	// MinVoltage The minimum voltage a grid must have.
	MinVoltage int16 = 1
	// MaxVoltage the maximum voltage a grid may have.
	MaxVoltage int16 = 400
	// MinPhases The minimum number of phases a grid must have
	MinPhases int8 = 1
	// MaxPhases The maximum number of phases a grid may have
	MaxPhases int8 = 3
	// MinCurrentPerPhase The minimum current per phase a grid must have.
	MinCurrentPerPhase float32 = 0.1
	// MaxCurrentPerPhase The maximum current per phase a grid may have.
	MaxCurrentPerPhase float32 = 100
)

// Grid Represents all properties a utility grid can have.
type Grid struct {
	voltage            int16
	phases             int8
	maxCurrentPerPhase float32
}

// Voltage Gives the voltage of the grid.
func (g *Grid) Voltage() int16 {
	return g.voltage
}

// setVoltage Sets the voltage of the grid.
func (g *Grid) setVoltage(voltage int16) error {
	if voltage < MinVoltage || voltage > MaxVoltage {
		return fmt.Errorf("grid voltage must be between %d and %d (inclusive), provided %d",
			MinVoltage, MaxVoltage, voltage)
	}
	g.voltage = voltage
	return nil
}

// Phases Gives the number of phases the grid has.
func (g *Grid) Phases() int8 {
	return g.phases
}

// setPhases Sets the number of phases in the grid. This value should be between MinPhases and MaxPhases (inclusive).
func (g *Grid) setPhases(phases int8) error {
	if phases < MinPhases || phases > MaxPhases {
		return fmt.Errorf("grid phases must be between %d and %d (inclusive), provided %d",
			MinPhases, MaxPhases, phases)
	}
	g.phases = phases
	return nil
}

// MaxCurrentPerPhase Gives the maximum current per phase the grid can provide.
func (g *Grid) MaxCurrentPerPhase() float32 {
	return g.maxCurrentPerPhase
}

// setMaxCurrentPerPhase Sets the maximum current per phase the grid can provide. This should be between
// MinCurrentPerPhase and MaxCurrentPerPhase (inclusive).
func (g *Grid) setMaxCurrentPerPhase(maxCurrentPerPhase float32) error {
	if maxCurrentPerPhase < MinCurrentPerPhase || maxCurrentPerPhase > MaxCurrentPerPhase {
		return fmt.Errorf("max current per phase should be between %f and %f (inclusive), provided %f",
			MinCurrentPerPhase, MaxCurrentPerPhase, maxCurrentPerPhase)
	}
	g.maxCurrentPerPhase = maxCurrentPerPhase
	return nil
}

func (g *Grid) MaxPowerPerPhase() int32 {
	return int32(g.MaxCurrentPerPhase() * float32(g.Voltage()))
}

func (g *Grid) MaxTotalPower() int32 {
	return g.MaxPowerPerPhase() * int32(g.Phases())
}

// NewGrid Constructs a new Grid instance with the given voltage, phases and max current
func NewGrid(voltage int16, phases int8, maxCurrentPerPhase float32) (*Grid, error) {
	var grid = Grid{}
	err := grid.setVoltage(voltage)
	if err != nil {
		return nil, err
	}
	err = grid.setPhases(phases)
	if err != nil {
		return nil, err
	}
	err = grid.setMaxCurrentPerPhase(maxCurrentPerPhase)
	if err != nil {
		return nil, err
	}
	return &grid, nil
}

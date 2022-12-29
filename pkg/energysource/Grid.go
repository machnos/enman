package energysource

import (
	"fmt"
)

type Grid interface {
	EnergyFlow
}

// GridBase Represents all live properties a utility grid can have.
type GridBase struct {
	*EnergyFlowBase
	gridConfig *GridConfig
}

func (gb *GridBase) ToMap() map[string]any {
	data := gb.EnergyFlowBase.ToMap()
	data["config"] = gb.gridConfig.ToMap()
	return data
}

// GridConfig Represents the static values a utility grid can have. For example the voltage the grid is running on.
type GridConfig struct {
	voltage            float32
	maxCurrentPerPhase float32
	phases             uint8
}

// MaxCurrentPerPhase Gives the maximum current per phase the grid can provide.
func (gc *GridConfig) MaxCurrentPerPhase() float32 {
	return gc.maxCurrentPerPhase
}

// setMaxCurrentPerPhase Sets the maximum current per phase the grid can provide. This should be between
// MinCurrentPerPhase and MaxCurrentPerPhase (inclusive).
func (gc *GridConfig) setMaxCurrentPerPhase(maxCurrentPerPhase float32) error {
	if maxCurrentPerPhase < MinCurrentPerPhase || maxCurrentPerPhase > MaxCurrentPerPhase {
		return fmt.Errorf("max current per phase should be between %f and %f (inclusive), provided %f",
			MinCurrentPerPhase, MaxCurrentPerPhase, maxCurrentPerPhase)
	}
	gc.maxCurrentPerPhase = maxCurrentPerPhase
	return nil
}

// Voltage Gives the voltage the grid is running on.
func (gc *GridConfig) Voltage() float32 {
	return gc.voltage
}

// SetVoltage Sets the voltage of the grid at a given line index.
func (gc *GridConfig) SetVoltage(voltage float32) error {
	if voltage < MinVoltage || voltage > MaxVoltage {
		return fmt.Errorf("grid voltage must be between %f and %f (inclusive), provided %f",
			MinVoltage, MaxVoltage, voltage)
	}
	gc.voltage = voltage
	return nil
}

// Phases Gives the number of phases the grid has.
func (gc *GridConfig) Phases() uint8 {
	return gc.phases
}

// SetPhases Sets the number of phases the grid has. This value should be between MinPhases and MaxPhases (inclusive).
func (gc *GridConfig) SetPhases(phases uint8) error {
	if phases < MinPhases || phases > MaxPhases {
		return fmt.Errorf("phases must be between %d and %d (inclusive), provided %d",
			MinPhases, MaxPhases, phases)
	}
	gc.phases = phases
	return nil
}

// MaxPowerPerPhase Calculates the maximum power (in watts) per phase.
func (gc *GridConfig) MaxPowerPerPhase() uint32 {
	return uint32(gc.MaxCurrentPerPhase() * gc.Voltage())
}

// MaxTotalPower Calculates the maximum total power (in watts) this grid can consume.
func (gc *GridConfig) MaxTotalPower() uint32 {
	return gc.MaxPowerPerPhase() * uint32(gc.Phases())
}

func (gc *GridConfig) ToMap() map[string]any {
	data := map[string]any{
		"phases":                gc.phases,
		"voltage":               gc.Voltage(),
		"max_current_per_phase": gc.MaxCurrentPerPhase(),
		"max_total_power":       gc.MaxTotalPower(),
	}
	return data
}

// NewGrid Constructs a new GridBase instance with the given voltage, phases and max current
func NewGrid(gridConfig *GridConfig) *GridBase {
	return &GridBase{
		EnergyFlowBase: &EnergyFlowBase{},
		gridConfig:     gridConfig,
	}
}

func NewGridConfig(voltage float32, maxCurrentPerPhase float32, phases uint8) (*GridConfig, error) {
	var gridConfig = GridConfig{}
	err := gridConfig.SetVoltage(voltage)
	if err != nil {
		return nil, err
	}
	err = gridConfig.setMaxCurrentPerPhase(maxCurrentPerPhase)
	if err != nil {
		return nil, err
	}
	err = gridConfig.SetPhases(phases)
	if err != nil {
		return nil, err
	}
	return &gridConfig, nil
}

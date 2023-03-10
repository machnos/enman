package energysource

import "fmt"

const (
	// MinVoltage The minimum voltage a grid must have.
	MinVoltage float32 = 1
	// MaxVoltage the maximum voltage a grid may have.
	MaxVoltage float32 = 600
	// MinCurrentPerPhase The minimum current per phase a grid must have.
	MinCurrentPerPhase float32 = 0.1
	// MaxCurrentPerPhase The maximum current per phase a grid may have.
	MaxCurrentPerPhase float32 = 100
	// MinPhases The minimum number of phases a grid must have
	MinPhases uint8 = 1
	// MaxPhases The maximum number of phases a grid may have
	MaxPhases uint8 = 3
)

type EnergyFlow interface {
	Phases() uint8
	Power(lineIx uint8) float32
	TotalPower() float32
	Voltage(lineIx uint8) float32
	Current(lineIx uint8) float32
	TotalCurrent() float32
	ToMap() map[string]any
}

type EnergyFlowBase struct {
	current [MaxPhases]float32
	power   [MaxPhases]float32
	voltage [MaxPhases]float32
}

func (efb *EnergyFlowBase) Phases() uint8 {
	for x := MaxPhases; x >= MinPhases; x-- {
		if efb.voltage[x-1] != 0 {
			return x
		}
	}
	return 0
}

func (efb *EnergyFlowBase) Power(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.power[lineIx]
}

// SetPower Sets the power of the grid at a given line index.
func (efb *EnergyFlowBase) SetPower(lineIx uint8, power float32) error {
	if !validLineIx(lineIx) {
		return fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	efb.power[lineIx] = power
	return nil
}

func (efb *EnergyFlowBase) TotalPower() float32 {
	totalPower := float32(0)
	for i := 0; i < len(efb.power); i++ {
		totalPower += efb.power[i]
	}
	return totalPower
}

func (efb *EnergyFlowBase) Voltage(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.voltage[lineIx]
}

// SetVoltage Sets the voltage of the grid at a given line index.
func (efb *EnergyFlowBase) SetVoltage(lineIx uint8, voltage float32) error {
	if !validLineIx(lineIx) {
		return fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	efb.voltage[lineIx] = voltage
	return nil
}

func (efb *EnergyFlowBase) Current(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.current[lineIx]
}

// SetCurrent Sets the current of the grid at a given line index.
func (efb *EnergyFlowBase) SetCurrent(lineIx uint8, current float32) error {
	if !validLineIx(lineIx) {
		return fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	efb.current[lineIx] = current
	return nil
}

func (efb *EnergyFlowBase) TotalCurrent() float32 {
	totalCurrent := float32(0)
	for i := 0; i < len(efb.current); i++ {
		totalCurrent += efb.current[i]
	}
	return totalCurrent
}

func (efb *EnergyFlowBase) ToMap() map[string]any {
	phases := efb.Phases()
	data := map[string]any{
		"phases":        phases,
		"total_current": efb.TotalCurrent(),
		"total_power":   efb.TotalPower(),
	}
	for ix := uint8(0); ix < phases; ix++ {
		data[fmt.Sprintf("l%d", ix)] = map[string]any{
			"voltage": efb.Voltage(ix),
			"current": efb.Current(ix),
			"power":   efb.Power(ix),
		}
	}
	return data
}

func validLineIx(lineIx uint8) bool {
	return lineIx < MaxPhases
}

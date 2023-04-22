package energysource

import (
	"fmt"
)

const (
	// MinVoltage The minimum voltage a grid must have.
	MinVoltage uint16 = 1
	// MaxVoltage the maximum voltage a grid may have.
	MaxVoltage uint16 = 600
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
	Name() string
	Role() string
	Phases() uint8
	Power(lineIx uint8) float32
	TotalPower() float32
	Voltage(lineIx uint8) float32
	Current(lineIx uint8) float32
	TotalCurrent() float32
	EnergyConsumed(lineIx uint8) float64
	TotalEnergyConsumed() float64
	EnergyProvided(lineIx uint8) float64
	TotalEnergyProvided() float64
}

type EnergyFlowUsage struct {
	energyConsumed      [MaxPhases]float64
	totalEnergyConsumed float64
	energyProvided      [MaxPhases]float64
	totalEnergyProvided float64
}

func NewEnergyFlowUsage() *EnergyFlowUsage {
	return &EnergyFlowUsage{}
}

func (efu *EnergyFlowUsage) EnergyConsumed(lineIx uint8) float64 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efu.energyConsumed[lineIx]
}

func (efu *EnergyFlowUsage) SetEnergyConsumed(lineIx uint8, energyConsumed float64) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efu.energyConsumed[lineIx] != energyConsumed
	efu.energyConsumed[lineIx] = energyConsumed
	return changed, nil
}

func (efu *EnergyFlowUsage) SetTotalEnergyConsumed(totalEnergyConsumed float64) bool {
	changed := efu.totalEnergyConsumed != totalEnergyConsumed
	efu.totalEnergyConsumed = totalEnergyConsumed
	return changed
}

func (efu *EnergyFlowUsage) TotalEnergyConsumed() float64 {
	if efu.totalEnergyConsumed != 0 {
		return efu.totalEnergyConsumed
	}
	totalEnergyConsumed := float64(0)
	for i := 0; i < len(efu.energyConsumed); i++ {
		totalEnergyConsumed += efu.energyConsumed[i]
	}
	return totalEnergyConsumed
}

func (efu *EnergyFlowUsage) EnergyProvided(lineIx uint8) float64 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efu.energyProvided[lineIx]
}

func (efu *EnergyFlowUsage) SetEnergyProvided(lineIx uint8, energyProvided float64) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efu.energyProvided[lineIx] != energyProvided
	efu.energyProvided[lineIx] = energyProvided
	return changed, nil
}

func (efu *EnergyFlowUsage) SetTotalEnergyProvided(totalEnergyProvided float64) bool {
	changed := efu.totalEnergyProvided != totalEnergyProvided
	efu.totalEnergyProvided = totalEnergyProvided
	return changed
}

func (efu *EnergyFlowUsage) TotalEnergyProvided() float64 {
	if efu.totalEnergyProvided != 0 {
		return efu.totalEnergyProvided
	}
	totalEnergyProvided := float64(0)
	for i := 0; i < len(efu.energyProvided); i++ {
		totalEnergyProvided += efu.energyProvided[i]
	}
	return totalEnergyProvided
}

type EnergyFlowState struct {
	current [MaxPhases]float32
	power   [MaxPhases]float32
	voltage [MaxPhases]float32
}

func NewEnergyFlowState() *EnergyFlowState {
	return &EnergyFlowState{}
}

func (efs *EnergyFlowState) Power(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efs.power[lineIx]
}

// SetPower Sets the power of the grid at a given line index.
func (efs *EnergyFlowState) SetPower(lineIx uint8, power float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efs.power[lineIx] != power
	efs.power[lineIx] = power
	return changed, nil
}

func (efs *EnergyFlowState) TotalPower() float32 {
	totalPower := float32(0)
	for i := 0; i < len(efs.power); i++ {
		totalPower += efs.power[i]
	}
	return totalPower
}

func (efs *EnergyFlowState) Voltage(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efs.voltage[lineIx]
}

// SetVoltage Sets the voltage of the grid at a given line index.
func (efs *EnergyFlowState) SetVoltage(lineIx uint8, voltage float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efs.voltage[lineIx] != voltage
	efs.voltage[lineIx] = voltage
	return changed, nil
}

func (efs *EnergyFlowState) Current(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efs.current[lineIx]
}

// SetCurrent Sets the current of the grid at a given line index.
func (efs *EnergyFlowState) SetCurrent(lineIx uint8, current float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efs.current[lineIx] != current
	efs.current[lineIx] = current
	return changed, nil
}

func (efs *EnergyFlowState) TotalCurrent() float32 {
	totalCurrent := float32(0)
	for i := 0; i < len(efs.current); i++ {
		totalCurrent += efs.current[i]
	}
	return totalCurrent
}

type EnergyFlowBase struct {
	*EnergyFlowUsage
	*EnergyFlowState
	name string
	role string
}

func NewEnergyFlowBase(name string, role string) *EnergyFlowBase {
	return &EnergyFlowBase{
		EnergyFlowUsage: NewEnergyFlowUsage(),
		EnergyFlowState: NewEnergyFlowState(),
		name:            name,
		role:            role,
	}
}

func (efb *EnergyFlowBase) Name() string {
	return efb.name
}

func (efb *EnergyFlowBase) Role() string {
	return efb.role
}

func (efb *EnergyFlowBase) Phases() uint8 {
	for x := MaxPhases; x >= MinPhases; x-- {
		if efb.voltage[x-1] != 0 {
			return x
		}
	}
	return 0
}

func validLineIx(lineIx uint8) bool {
	return lineIx < MaxPhases
}

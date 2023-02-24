package energysource

import "fmt"

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
	ToMap() map[string]any
}

type EnergyFlowBase struct {
	EnergyFlow
	name                string
	current             [MaxPhases]float32
	power               [MaxPhases]float32
	voltage             [MaxPhases]float32
	energyConsumed      [MaxPhases]float64
	totalEnergyConsumed float64
	energyProvided      [MaxPhases]float64
	totalEnergyProvided float64
}

func (efb *EnergyFlowBase) Name() string {
	return efb.name
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
func (efb *EnergyFlowBase) SetPower(lineIx uint8, power float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efb.power[lineIx] != power
	efb.power[lineIx] = power
	return changed, nil
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
func (efb *EnergyFlowBase) SetVoltage(lineIx uint8, voltage float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efb.voltage[lineIx] != voltage
	efb.voltage[lineIx] = voltage
	return changed, nil
}

func (efb *EnergyFlowBase) Current(lineIx uint8) float32 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.current[lineIx]
}

// SetCurrent Sets the current of the grid at a given line index.
func (efb *EnergyFlowBase) SetCurrent(lineIx uint8, current float32) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efb.current[lineIx] != current
	efb.current[lineIx] = current
	return changed, nil
}

func (efb *EnergyFlowBase) TotalCurrent() float32 {
	totalCurrent := float32(0)
	for i := 0; i < len(efb.current); i++ {
		totalCurrent += efb.current[i]
	}
	return totalCurrent
}

func (efb *EnergyFlowBase) EnergyConsumed(lineIx uint8) float64 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.energyConsumed[lineIx]
}

func (efb *EnergyFlowBase) SetEnergyConsumed(lineIx uint8, energyConsumed float64) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efb.energyConsumed[lineIx] != energyConsumed
	efb.energyConsumed[lineIx] = energyConsumed
	return changed, nil
}

func (efb *EnergyFlowBase) SetTotalEnergyConsumed(totalEnergyConsumed float64) bool {
	changed := efb.totalEnergyConsumed != totalEnergyConsumed
	efb.totalEnergyConsumed = totalEnergyConsumed
	return changed
}

func (efb *EnergyFlowBase) TotalEnergyConsumed() float64 {
	if efb.totalEnergyConsumed != 0 {
		return efb.totalEnergyConsumed
	}
	totalEnergyConsumed := float64(0)
	for i := 0; i < len(efb.energyConsumed); i++ {
		totalEnergyConsumed += efb.energyConsumed[i]
	}
	return totalEnergyConsumed
}

func (efb *EnergyFlowBase) EnergyProvided(lineIx uint8) float64 {
	if !validLineIx(lineIx) {
		return 0
	}
	return efb.energyProvided[lineIx]
}

func (efb *EnergyFlowBase) SetEnergyProvided(lineIx uint8, energyProvided float64) (bool, error) {
	if !validLineIx(lineIx) {
		return false, fmt.Errorf("lineIx must be between %d and %d (inclusive), provided %d",
			MinPhases-1, MaxPhases-1, lineIx)
	}
	changed := efb.energyProvided[lineIx] != energyProvided
	efb.energyProvided[lineIx] = energyProvided
	return changed, nil
}

func (efb *EnergyFlowBase) SetTotalEnergyProvided(totalEnergyProvided float64) bool {
	changed := efb.totalEnergyProvided != totalEnergyProvided
	efb.totalEnergyProvided = totalEnergyProvided
	return changed
}

func (efb *EnergyFlowBase) TotalEnergyProvided() float64 {
	if efb.totalEnergyProvided != 0 {
		return efb.totalEnergyProvided
	}
	totalEnergyProvided := float64(0)
	for i := 0; i < len(efb.energyProvided); i++ {
		totalEnergyProvided += efb.energyProvided[i]
	}
	return totalEnergyProvided
}

func (efb *EnergyFlowBase) ToMap() map[string]any {
	phases := efb.Phases()
	data := map[string]any{
		"name":                  efb.Name(),
		"phases":                phases,
		"total_current":         efb.TotalCurrent(),
		"total_power":           efb.TotalPower(),
		"total_energy_consumed": efb.TotalEnergyConsumed(),
		"total_energy_provided": efb.TotalEnergyProvided(),
	}
	for ix := uint8(0); ix < phases; ix++ {
		data[fmt.Sprintf("l%d", ix)] = map[string]any{
			"voltage":         efb.Voltage(ix),
			"current":         efb.Current(ix),
			"power":           efb.Power(ix),
			"energy_consumed": efb.EnergyConsumed(ix),
			"energy_provided": efb.EnergyProvided(ix),
		}
	}
	return data
}

func validLineIx(lineIx uint8) bool {
	return lineIx < MaxPhases
}

package electricity

import (
	"encoding/json"
	gpv "github.com/go-playground/validator/v10"
)

var validator = gpv.New()

const (
	// MinPhases The minimum number of phases a grid must have
	MinPhases uint8 = 1
	// MaxPhases The maximum number of phases a grid may have
	MaxPhases uint8 = 3
)

type Usage struct {
	energyConsumed      [MaxPhases]float64
	totalEnergyConsumed float64
	energyProvided      [MaxPhases]float64
	totalEnergyProvided float64
}

type usage struct {
	EnergyConsumed      [MaxPhases]float64 `json:"energy_consumed" validate:"dive,gte=0"`
	TotalEnergyConsumed float64            `json:"total_energy_consumed" validate:"gte=0"`
	EnergyProvided      [MaxPhases]float64 `json:"energy_provided" validate:"dive,gte=0"`
	TotalEnergyProvided float64            `json:"total_energy_provided" validate:"gte=0"`
}

func NewUsage() *Usage {
	return &Usage{
		energyConsumed: [3]float64(make([]float64, MaxPhases)),
		energyProvided: [3]float64(make([]float64, MaxPhases)),
	}
}

func (u *Usage) EnergyConsumed(lineIx uint8) float64 {
	return u.energyConsumed[lineIx]
}

func (u *Usage) SetEnergyConsumed(lineIx uint8, energyConsumed float64) *Usage {
	u.energyConsumed[lineIx] = energyConsumed
	return u
}

func (u *Usage) SetTotalEnergyConsumed(totalEnergyConsumed float64) *Usage {
	u.totalEnergyConsumed = totalEnergyConsumed
	return u
}

func (u *Usage) TotalEnergyConsumed() float64 {
	if u.totalEnergyConsumed != 0 {
		return u.totalEnergyConsumed
	}
	totalEnergyConsumed := float64(0)
	for i := 0; i < len(u.energyConsumed); i++ {
		totalEnergyConsumed += u.energyConsumed[i]
	}
	return totalEnergyConsumed
}

func (u *Usage) EnergyProvided(lineIx uint8) float64 {
	return u.energyProvided[lineIx]
}

func (u *Usage) SetEnergyProvided(lineIx uint8, energyProvided float64) *Usage {
	u.energyProvided[lineIx] = energyProvided
	return u
}

func (u *Usage) SetTotalEnergyProvided(totalEnergyProvided float64) *Usage {
	u.totalEnergyProvided = totalEnergyProvided
	return u
}

func (u *Usage) TotalEnergyProvided() float64 {
	if u.totalEnergyProvided != 0 {
		return u.totalEnergyProvided
	}
	totalEnergyProvided := float64(0)
	for i := 0; i < len(u.energyProvided); i++ {
		totalEnergyProvided += u.energyProvided[i]
	}
	return totalEnergyProvided
}

func (u *Usage) IsZero() bool {
	if u.totalEnergyConsumed != 0 || u.totalEnergyProvided != 0 {
		return false
	}
	for _, value := range u.energyConsumed {
		if value != 0 {
			return false
		}
	}
	for _, value := range u.energyProvided {
		if value != 0 {
			return false
		}
	}
	return true
}

func (u *Usage) SetValues(other *Usage) {
	u.energyConsumed = other.energyConsumed
	u.energyProvided = other.energyProvided
	u.totalEnergyConsumed = other.totalEnergyConsumed
	u.totalEnergyProvided = other.totalEnergyProvided
}

func (u *Usage) rawValues() usage {
	return usage{u.energyConsumed,
		u.TotalEnergyConsumed(),
		u.energyProvided,
		u.TotalEnergyProvided()}
}

func (u *Usage) Valid() (bool, error) {
	err := validator.Struct(u.rawValues())
	if err != nil {
		return false, err
	}
	return true, nil
}

func (u *Usage) MarshalJSON() ([]byte, error) {
	rawValues := u.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

type State struct {
	current [MaxPhases]float32
	power   [MaxPhases]float32
	voltage [MaxPhases]float32
}

type state struct {
	Current      [MaxPhases]float32 `json:"current"`
	TotalCurrent float32            `json:"total_current"`
	Power        [MaxPhases]float32 `json:"power"`
	TotalPower   float32            `json:"total_power"`
	Voltage      [MaxPhases]float32 `json:"voltage" validate:"dive,gte=0"`
}

func NewState() *State {
	return &State{
		[MaxPhases]float32(make([]float32, MaxPhases)),
		[MaxPhases]float32(make([]float32, MaxPhases)),
		[MaxPhases]float32(make([]float32, MaxPhases)),
	}
}

func (s *State) Power(lineIx uint8) float32 {
	return s.power[lineIx]
}

func (s *State) SetPower(lineIx uint8, power float32) *State {
	s.power[lineIx] = power
	return s
}

func (s *State) TotalPower() float32 {
	totalPower := float32(0)
	for i := 0; i < len(s.power); i++ {
		totalPower += s.power[i]
	}
	return totalPower
}

func (s *State) Voltage(lineIx uint8) float32 {
	return s.voltage[lineIx]
}

func (s *State) SetVoltage(lineIx uint8, voltage float32) *State {
	s.voltage[lineIx] = voltage
	return s
}

func (s *State) Current(lineIx uint8) float32 {
	return s.current[lineIx]
}

func (s *State) SetCurrent(lineIx uint8, current float32) *State {
	s.current[lineIx] = current
	return s
}

func (s *State) TotalCurrent() float32 {
	totalCurrent := float32(0)
	for i := 0; i < len(s.current); i++ {
		totalCurrent += s.current[i]
	}
	return totalCurrent
}

func (s *State) Phases() uint8 {
	phases := uint8(0)
	for x := MinPhases - 1; x < MaxPhases; x++ {
		if s.power[x] != 0 || s.voltage[x] != 0 || s.current[x] != 0 {
			phases++
		}
	}
	return phases
}

func (s *State) SetValues(other *State) {
	s.power = other.power
	s.current = other.current
	s.voltage = other.voltage
}

func (s *State) rawValues() state {
	return state{s.current,
		s.TotalCurrent(),
		s.power,
		s.TotalPower(),
		s.voltage}
}

func (s *State) Valid() (bool, error) {
	rawValues := s.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *State) MarshalJSON() ([]byte, error) {
	rawValues := s.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (s *State) IsZero() bool {
	for voltage := range s.voltage {
		if voltage != 0 {
			return false
		}
	}
	for current := range s.current {
		if current != 0 {
			return false
		}
	}
	for power := range s.power {
		if power != 0 {
			return false
		}
	}
	return true
}

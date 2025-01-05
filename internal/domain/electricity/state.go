package electricity

type State struct {
	current [MaxPhases]float32
	power   [MaxPhases]float32
	voltage [MaxPhases]float32
}

type state struct {
	Current      [MaxPhases]float32
	TotalCurrent float32
	Power        [MaxPhases]float32
	TotalPower   float32
	Voltage      [MaxPhases]float32 `validate:"dive,gte=0"`
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

package battery

import (
	"encoding/json"
	gpv "github.com/go-playground/validator/v10"
)

var validator = gpv.New()

type State struct {
	current float32
	power   float32
	voltage float32
	soc     float32
	soh     float32
}

type state struct {
	Current float32 `json:"current"`
	Power   float32 `json:"power"`
	Voltage float32 `json:"voltage" validate:"gte=0"`
	SoC     float32 `json:"soc" validate:"gte=0"`
	SoH     float32 `json:"soh" validate:"gte=0"`
}

func NewState() *State {
	return &State{}
}

func (bs *State) Current() float32 {
	return bs.current
}

func (bs *State) SetCurrent(current float32) {
	bs.current = current
}

func (bs *State) Power() float32 {
	return bs.power
}

func (bs *State) SetPower(power float32) {
	bs.power = power
}

func (bs *State) Voltage() float32 {
	return bs.voltage
}

func (bs *State) SetVoltage(voltage float32) {
	bs.voltage = voltage
}

func (bs *State) SoC() float32 {
	return bs.soc
}

func (bs *State) SetSoC(soc float32) {
	bs.soc = soc
}

func (bs *State) SoH() float32 {
	return bs.soh
}

func (bs *State) SetSoH(soh float32) {
	bs.soh = soh
}

func (bs *State) SetValues(other *State) {
	bs.current = other.current
	bs.power = other.power
	bs.voltage = other.voltage
	bs.soc = other.soc
	bs.soh = other.soh
}

func (bs *State) rawValues() state {
	return state{
		bs.current,
		bs.power,
		bs.voltage,
		bs.soc,
		bs.soh,
	}
}

func (bs *State) Valid() (bool, error) {
	rawValues := bs.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (bs *State) MarshalJSON() ([]byte, error) {
	rawValues := bs.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(rawValues)
}

func (bs *State) IsZero() bool {
	if bs.current != 0 {
		return false
	}
	if bs.power != 0 {
		return false
	}
	if bs.voltage != 0 {
		return false
	}
	return true
}

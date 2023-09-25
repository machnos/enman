package domain

import (
	"time"
)

type EnergySourceRole string

const (
	RoleGrid      EnergySourceRole = "Grid"
	RolePv        EnergySourceRole = "Pv"
	RoleBattery   EnergySourceRole = "Battery"
	RoleEvCharger EnergySourceRole = "EvCharger"
)

type System struct {
	location *time.Location
	grid     *Grid
	pvs      []*Pv
}

func NewSystem(location *time.Location) *System {
	return &System{
		location: location,
	}
}

func (s *System) Location() *time.Location {
	return s.location
}

func (s *System) SetGrid(name string, voltage uint16, maxCurrentPerPhase float32, phases uint8) *System {
	if s.grid != nil {
		ElectricityMeterReadings.Deregister(&GridElectricityMeterListener{grid: s.grid})
	}
	s.grid = &Grid{
		name:               name,
		voltage:            voltage,
		maxCurrentPerPhase: maxCurrentPerPhase,
		phases:             phases,
		electricityState:   NewElectricityState(),
		electricityUsage:   NewElectricityUsage(),
		gasUsage:           NewGasUsage(),
		waterUsage:         NewWaterUsage(),
	}
	ElectricityMeterReadings.Register(&GridElectricityMeterListener{grid: s.grid}, func(values *ElectricityMeterValues) bool {
		return s.grid.name == values.Name() && RoleGrid == values.Role()
	})
	GasMeterReadings.Register(&GridGasMeterListener{grid: s.grid}, func(values *GasMeterValues) bool {
		return s.grid.name == values.Name() && RoleGrid == values.Role()
	})
	WaterMeterReadings.Register(&GridWaterMeterListener{grid: s.grid}, func(values *WaterMeterValues) bool {
		return s.grid.name == values.Name() && RoleGrid == values.Role()
	})
	return s
}

func (s *System) Grid() *Grid {
	return s.grid
}

func (s *System) AddPv(name string) *System {
	pv := &Pv{
		name:             name,
		electricityState: NewElectricityState(),
		electricityUsage: NewElectricityUsage(),
	}
	ElectricityMeterReadings.Register(&PvMeterListener{pv: pv}, func(values *ElectricityMeterValues) bool {
		return pv.name == values.Name() && RolePv == values.Role()
	})
	s.pvs = append(s.pvs, pv)
	return s
}

func (s *System) Pvs() []*Pv {
	return s.pvs
}

type ElectricitySource interface {
	Pv | Grid
	Name() string
	ElectricityState() *ElectricityState
	ElectricityUsage() *ElectricityUsage
}

type Grid struct {
	name               string
	voltage            uint16
	maxCurrentPerPhase float32
	phases             uint8
	electricityState   *ElectricityState
	electricityUsage   *ElectricityUsage
	gasUsage           *GasUsage
	waterUsage         *WaterUsage
}

func (g *Grid) Name() string {
	return g.name
}

func (g *Grid) Voltage() uint16 {
	return g.voltage
}

func (g *Grid) MaxCurrentPerPhase() float32 {
	return g.maxCurrentPerPhase
}

func (g *Grid) Phases() uint8 {
	return g.phases
}

func (g *Grid) ElectricityState() *ElectricityState {
	return g.electricityState
}

func (g *Grid) ElectricityUsage() *ElectricityUsage {
	return g.electricityUsage
}

type GridElectricityMeterListener struct {
	grid *Grid
}

func (geml *GridElectricityMeterListener) HandleEvent(values *ElectricityMeterValues) {
	if values.ElectricityState() != nil {
		geml.grid.electricityState.SetValues(values.ElectricityState())
	}
	if values.electricityUsage != nil {
		geml.grid.electricityUsage.SetValues(values.ElectricityUsage())
	}
}

type GridGasMeterListener struct {
	grid *Grid
}

func (ggml *GridGasMeterListener) HandleEvent(values *GasMeterValues) {
	if values.GasUsage() != nil {
		ggml.grid.gasUsage.SetValues(values.GasUsage())
	}
}

type GridWaterMeterListener struct {
	grid *Grid
}

func (gwml *GridWaterMeterListener) HandleEvent(values *WaterMeterValues) {
	if values.WaterUsage() != nil {
		gwml.grid.waterUsage.SetValues(values.WaterUsage())
	}
}

type Pv struct {
	name             string
	electricityState *ElectricityState
	electricityUsage *ElectricityUsage
}

func (p *Pv) Name() string {
	return p.name
}

func (p *Pv) ElectricityState() *ElectricityState {
	return p.electricityState
}

func (p *Pv) ElectricityUsage() *ElectricityUsage {
	return p.electricityUsage
}

type PvMeterListener struct {
	pv *Pv
}

func (pvml *PvMeterListener) HandleEvent(values *ElectricityMeterValues) {
	pvml.pv.electricityState.SetValues(values.ElectricityState())
	if values.electricityUsage != nil {
		pvml.pv.electricityUsage.SetValues(values.ElectricityUsage())
	}
}

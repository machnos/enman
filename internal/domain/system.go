package domain

import (
	"time"
)

type ElectricitySourceRole string

const (
	RoleGrid ElectricitySourceRole = "Grid"
	RolePv   ElectricitySourceRole = "Pv"
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
		ElectricityMeterReadings.Deregister(&GridMeterListener{grid: s.grid})
	}
	s.grid = &Grid{
		name:               name,
		voltage:            voltage,
		maxCurrentPerPhase: maxCurrentPerPhase,
		phases:             phases,
		electricityState:   NewElectricityState(),
		electricityUsage:   NewElectricityUsage(),
	}
	ElectricityMeterReadings.Register(&GridMeterListener{grid: s.grid}, func(values *ElectricityMeterValues) bool {
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

type GridMeterListener struct {
	grid *Grid
}

func (gml *GridMeterListener) HandleEvent(values *ElectricityMeterValues) {
	gml.grid.electricityState.SetValues(values.ElectricityState())
	if values.electricityUsage != nil {
		gml.grid.electricityUsage.SetValues(values.ElectricityUsage())
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

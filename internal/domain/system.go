package domain

import (
	"context"
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
	location  *time.Location
	grid      *Grid
	pvs       []*Pv
	acLoads   []*AcLoad
	batteries []*Battery
}

func NewSystem(location *time.Location) *System {
	return &System{
		location: location,
	}
}

func (s *System) Location() *time.Location {
	return s.location
}

func (s *System) SetGrid(name string, voltage uint16, maxCurrentPerPhase float32, phases uint8, meters []EnergyMeter) *System {
	s.grid = &Grid{
		name:               name,
		voltage:            voltage,
		maxCurrentPerPhase: maxCurrentPerPhase,
		phases:             phases,
		meters:             meters,
	}
	return s
}

func (s *System) Grid() *Grid {
	return s.grid
}

func (s *System) AddPv(name string, meters []EnergyMeter) *System {
	s.pvs = append(s.pvs, &Pv{
		name:   name,
		meters: meters,
	})
	return s
}

func (s *System) Pvs() []*Pv {
	return s.pvs
}

func (s *System) AcLoads() []*AcLoad {
	return s.acLoads
}

func (s *System) AddAcLoad(name string, role EnergySourceRole, meters []EnergyMeter) *System {
	s.acLoads = append(s.acLoads, &AcLoad{
		name:   name,
		role:   role,
		meters: meters,
	})
	return s
}

func (s *System) Batteries() []*Battery {
	return s.batteries
}

func (s *System) AddBattery(name string, meters []EnergyMeter) *System {
	s.batteries = append(s.batteries, &Battery{
		name:   name,
		meters: meters,
	})
	return s
}

func (s *System) StartMeasuring(context context.Context) {
	if s.grid != nil {
		s.grid.StartMeasuring(context)
	}
	for _, pv := range s.Pvs() {
		pv.StartMeasuring(context)
	}
	for _, acLoad := range s.AcLoads() {
		acLoad.StartMeasuring(context)
	}
	for _, battery := range s.Batteries() {
		battery.StartMeasuring(context)
	}
}

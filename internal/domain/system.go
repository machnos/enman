package domain

import (
	"context"
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/gas"
	"enman/internal/domain/water"
	"time"
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

func (s *System) SetGrid(name string, voltage uint16, maxCurrentPerPhase float32, phases uint8, electricityTargetConsumption int, meters []EnergyMeter, controller GridController) *System {
	s.grid = &Grid{
		name:                         name,
		voltage:                      voltage,
		maxCurrentPerPhase:           maxCurrentPerPhase,
		phases:                       phases,
		electricityTargetConsumption: electricityTargetConsumption,
		meters:                       meters,
		controller:                   controller,
		electricityState:             electricity.NewState(),
		electricityUsage:             electricity.NewUsage(),
		gasUsage:                     gas.NewUsage(),
		waterUsage:                   water.NewUsage(),
	}
	return s
}

func (s *System) Grid() *Grid {
	return s.grid
}

func (s *System) AddPv(name string, meters []EnergyMeter, controller PvController) *System {
	s.pvs = append(s.pvs, &Pv{
		name:       name,
		meters:     meters,
		controller: controller,
		state:      electricity.NewState(),
		usage:      electricity.NewUsage(),
	})
	return s
}

func (s *System) Pvs() []*Pv {
	return s.pvs
}

func (s *System) AcLoads() []*AcLoad {
	return s.acLoads
}

func (s *System) AddAcLoad(name string, role constants.EnergySourceRole, percentageFromGrid uint8, meters []EnergyMeter) *System {
	s.acLoads = append(s.acLoads, &AcLoad{
		name:               name,
		role:               role,
		percentageFromGrid: percentageFromGrid,
		meters:             meters,
		state:              electricity.NewState(),
		usage:              electricity.NewUsage(),
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
		state:  battery.NewState(),
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
	for _, b := range s.Batteries() {
		b.StartMeasuring(context)
	}
}

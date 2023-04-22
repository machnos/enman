package internal

import (
	"enman/internal/energysource"
	"time"
)

type System struct {
	location *time.Location
	grid     energysource.Grid
	pvs      []energysource.Pv
}

func NewSystem(location *time.Location) *System {
	return &System{
		location: location,
	}
}

func (s *System) Location() *time.Location {
	return s.location
}

func (s *System) Grid() energysource.Grid {
	return s.grid
}

func (s *System) SetGrid(grid energysource.Grid) {
	s.grid = grid
}

func (s *System) Pvs() []energysource.Pv {
	return s.pvs
}

func (s *System) AddPv(p energysource.Pv) {
	s.pvs = append(s.pvs, p)
}

type UpdateChannels struct {
	gridUpdated chan energysource.Grid
	pvUpdated   chan energysource.Pv
}

func (u *UpdateChannels) GridUpdated() chan energysource.Grid {
	return u.gridUpdated
}

func (u *UpdateChannels) PvUpdated() chan energysource.Pv {
	return u.pvUpdated
}

func NewUpdateChannels() *UpdateChannels {
	return &UpdateChannels{
		gridUpdated: make(chan energysource.Grid),
		pvUpdated:   make(chan energysource.Pv),
	}
}

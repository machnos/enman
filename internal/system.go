package internal

import (
	"enman/internal/energysource"
)

type System struct {
	grid energysource.Grid
	pvs  []energysource.Pv
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

func (s *System) ToMap() map[string]any {
	data := map[string]any{}
	if s.Grid() != nil {
		data["grid"] = s.Grid().ToMap()
	}
	if s.Pvs() != nil {
		var pvData []map[string]any
		for ix := 0; ix < len(s.Pvs()); ix++ {
			pvData = append(pvData, s.Pvs()[ix].ToMap())
		}
		data["pvs"] = pvData
	}
	return data
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

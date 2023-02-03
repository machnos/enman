package energysource

type System struct {
	grid Grid
	pvs  []Pv
}

func (s *System) Grid() Grid {
	return s.grid
}

func (s *System) Pvs() []Pv {
	return s.pvs
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

func (s *System) Merge(system *System) {
	if s.grid != nil {
		s.grid = system.Grid()
	}
	if system.Pvs() != nil {
		if s.pvs == nil {
			s.pvs = system.Pvs()
		} else {
			s.pvs = append(s.pvs, system.Pvs()...)
		}
	}
}

func NewSystem(grid Grid, pvs []Pv) *System {
	system := &System{
		grid: grid,
		pvs:  pvs,
	}
	return system
}

type UpdateChannels struct {
	gridUpdated chan Grid
	pvUpdated   chan Pv
}

func (u *UpdateChannels) GridUpdated() chan Grid {
	return u.gridUpdated
}

func (u *UpdateChannels) PvUpdated() chan Pv {
	return u.pvUpdated
}

func NewUpdateChannels() *UpdateChannels {
	return &UpdateChannels{
		gridUpdated: make(chan Grid),
		pvUpdated:   make(chan Pv),
	}
}

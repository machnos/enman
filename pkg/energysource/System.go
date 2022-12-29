package energysource

type System struct {
	grid *Grid
	pvs  []*Pv
}

func (s *System) Grid() *Grid {
	return s.grid
}

func (s *System) Pvs() []*Pv {
	return s.pvs
}

func (s *System) ToMap() map[string]any {
	data := map[string]any{}
	if s.Grid() != nil {
		data["grid"] = (*s.Grid()).ToMap()
	}
	if s.Pvs() != nil {
		var pvData []map[string]any
		for ix := 0; ix < len(s.Pvs()); ix++ {
			pvData = append(pvData, (*s.Pvs()[ix]).ToMap())
		}
		data["pvs"] = pvData
	}
	return data
}

func NewSystem(grid *Grid, pvs []*Pv) *System {
	return &System{
		grid: grid,
		pvs:  pvs,
	}
}

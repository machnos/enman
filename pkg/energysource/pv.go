package energysource

type Pv interface {
	EnergyFlow
}

type PvBase struct {
	*EnergyFlowBase
	pvConfig *PvConfig
}

func (p *PvBase) ToMap() map[string]any {
	data := p.EnergyFlowBase.ToMap()
	data["config"] = p.pvConfig.ToMap()
	data["config"] = nil
	return data
}

type PvConfig struct {
	name string
}

func (pvc *PvConfig) ToMap() map[string]any {
	data := map[string]any{
		"name": pvc.name,
	}
	return data
}

func NewPvBase(pvConfig *PvConfig) *PvBase {
	return &PvBase{
		EnergyFlowBase: &EnergyFlowBase{
			name: pvConfig.name,
		},
		pvConfig: pvConfig,
	}
}

func NewPvConfig(name string) *PvConfig {
	return &PvConfig{
		name: name,
	}
}

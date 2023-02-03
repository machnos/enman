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
}

func (pvc *PvConfig) ToMap() map[string]any {
	data := map[string]any{}
	return data
}

func NewPvBase(name string, pvConfig *PvConfig) *PvBase {
	return &PvBase{
		EnergyFlowBase: &EnergyFlowBase{
			name: name,
		},
		pvConfig: pvConfig,
	}
}

func NewPvConfig() *PvConfig {
	return &PvConfig{}
}

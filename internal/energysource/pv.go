package energysource

type Pv interface {
	EnergyFlow
}

type PvBase struct {
	*EnergyFlowBase
	pvConfig *PvConfig
}

type PvConfig struct {
}

func (pvc *PvConfig) ToMap() map[string]any {
	data := map[string]any{}
	return data
}

func NewPvBase(name string, pvConfig *PvConfig) *PvBase {
	return &PvBase{
		EnergyFlowBase: NewEnergyFlowBase(name, "pv"),
		pvConfig:       pvConfig,
	}
}

func NewPvConfig() *PvConfig {
	return &PvConfig{}
}

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

func NewPvBase(pvConfig *PvConfig) *PvBase {
	return &PvBase{
		EnergyFlowBase: &EnergyFlowBase{},
		pvConfig:       pvConfig,
	}
}

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

func NewPv(pvConfig *PvConfig) *PvBase {
	return &PvBase{
		EnergyFlowBase: &EnergyFlowBase{},
		pvConfig:       pvConfig,
	}
}

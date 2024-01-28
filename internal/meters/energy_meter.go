package meters

import (
	"enman/internal/config"
	"enman/internal/domain"
)

type energyMeter struct {
	brand  string
	model  string
	serial string
}

func (em *energyMeter) Brand() string {
	return em.brand
}

func (em *energyMeter) Model() string {
	return em.model
}

func (em *energyMeter) Serial() string {
	return em.serial
}

func newEnergyMeter(brand string) *energyMeter {
	return &energyMeter{
		brand: brand,
	}
}

func ProbeEnergyMeters(name string, role domain.EnergySourceRole, meterConfigs []*config.EnergyMeter) []domain.EnergyMeter {
	meters := make([]domain.EnergyMeter, 0)
	for _, meterConfig := range meterConfigs {
		meter := probeEnergyMeter(name, role, meterConfig)
		if meter != nil {
			meters = append(meters, meter)
		}
	}
	return meters
}

func probeEnergyMeter(name string, role domain.EnergySourceRole, meterConfig *config.EnergyMeter) domain.EnergyMeter {
	if "modbus" == meterConfig.Type {
		return probeModbusMeter(name, role, meterConfig)
	} else if "serial" == meterConfig.Type {
		return probeSerialMeter(name, role, meterConfig)
	}
	return nil
}

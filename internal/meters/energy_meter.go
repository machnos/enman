package meters

import (
	"context"
	"enman/internal/config"
	"enman/internal/domain"
	"time"
)

type energyMeter struct {
	name         string
	role         domain.EnergySourceRole
	brand        string
	model        string
	serial       string
	meter        implementingEnergyMeter
	updateTicker *time.Ticker
}

func (em *energyMeter) Name() string {
	return em.name
}

func (em *energyMeter) Role() domain.EnergySourceRole {
	return em.role
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

func newEnergyMeter(name string, role domain.EnergySourceRole) *energyMeter {
	return &energyMeter{
		name: name,
		role: role,
	}
}

func (em *energyMeter) StartReading(context context.Context) {
	if em.updateTicker != nil {
		// Meter already started
		return
	}
	em.updateTicker = time.NewTicker(em.meter.updateInterval())

	go func() {
		for {
			select {
			case <-context.Done():
				em.meter.shutdown()
				return
			case _ = <-em.updateTicker.C:
				electricityState := domain.NewElectricityState()
				electricityUsage := domain.NewElectricityUsage()
				gasUsage := domain.NewGasUsage()
				waterUsage := domain.NewWaterUsage()

				em.meter.readValues(electricityState, electricityUsage, gasUsage, waterUsage)
				var electricityMeterValues *domain.ElectricityMeterValues
				var gasMeterValues *domain.GasMeterValues
				var waterMeterValues *domain.WaterMeterValues
				if !electricityState.IsZero() || !electricityUsage.IsZero() {
					electricityMeterValues = domain.NewElectricityMeterValues().
						SetName(em.name).
						SetRole(em.role).
						SetElectricityState(electricityState).
						SetElectricityUsage(electricityUsage)
				}
				if !gasUsage.IsZero() {
					gasMeterValues = domain.NewGasMeterValues().
						SetName(em.name).
						SetRole(em.role).
						SetGasUsage(gasUsage)
				}
				if !waterUsage.IsZero() {
					waterMeterValues = domain.NewWaterMeterValues().
						SetName(em.name).
						SetRole(em.role).
						SetWaterUsage(waterUsage)
				}
				em.meter.enrichEvents(electricityMeterValues, gasMeterValues, waterMeterValues)
				if !electricityState.IsZero() || !electricityUsage.IsZero() {
					domain.ElectricityMeterReadings.Trigger(electricityMeterValues)
				}
				if !gasUsage.IsZero() {
					domain.GasMeterReadings.Trigger(gasMeterValues)
				}
				if !waterUsage.IsZero() {
					domain.WaterMeterReadings.Trigger(waterMeterValues)
				}
			}
		}
	}()
}

type implementingEnergyMeter interface {
	updateInterval() time.Duration
	readValues(*domain.ElectricityState, *domain.ElectricityUsage, *domain.GasUsage, *domain.WaterUsage)
	enrichEvents(*domain.ElectricityMeterValues, *domain.GasMeterValues, *domain.WaterMeterValues)
	shutdown()
}

func ProbeEnergyMeter(name string, role domain.EnergySourceRole, meterConfigs []*config.EnergyMeter) domain.EnergyMeter {
	if len(meterConfigs) > 1 {
		// TODO implement compound energy meters
	}
	return probeEnergyMeter(name, role, meterConfigs[0])
}

func probeEnergyMeter(name string, role domain.EnergySourceRole, meterConfig *config.EnergyMeter) domain.EnergyMeter {
	if "modbus" == meterConfig.Type {
		return probeModbusMeter(name, role, meterConfig)
	} else if "serial" == meterConfig.Type {
		return probeSerialMeter(name, role, meterConfig)
	}
	return nil
}

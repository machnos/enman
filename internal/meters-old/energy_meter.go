package meters_old

import (
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"time"
)

type implementingEnergyMeter interface {
	// waitForInitialization waits until the electricity meter is fully initialized and returns true or false
	// whether the meter is capable of reading values.
	waitForInitialization() bool
	readValues(*domain.ElectricityState, *domain.ElectricityUsage, *domain.GasUsage, *domain.WaterUsage)
	enrichMeterValues(*domain.ElectricityMeterValues, *domain.GasMeterValues, *domain.WaterMeterValues)
	shutdown(name string)
	updateInterval() time.Duration
}

type genericEnergyMeter struct {
	implementingEnergyMeter
	name          string
	updateTicker  *time.Ticker
	usageLastRead time.Time
}

func (gem *genericEnergyMeter) Name() string {
	return gem.name
}

func (gem *genericEnergyMeter) StartReading(ctx context.Context) {
	if !gem.waitForInitialization() {
		log.Warningf("Unable to read values from energy meter %s as it is an unknown device", gem.name)
		return
	}

	gem.updateTicker = time.NewTicker(gem.updateInterval())

	go func() {
		for {
			select {
			case <-ctx.Done():
				if gem.updateTicker != nil {
					gem.updateTicker.Stop()
					gem.updateTicker = nil
					log.Infof("Stop reading values from energy meter %s", gem.name)
				}
				gem.shutdown(gem.name)
				return
			case _ = <-gem.updateTicker.C:
				electricityState := domain.NewElectricityState()
				electricityUsage := domain.NewElectricityUsage()
				gasUsage := domain.NewGasUsage()
				waterUsage := domain.NewWaterUsage()

				gem.readValues(electricityState, electricityUsage, gasUsage, waterUsage)

				var electricityMeterValues *domain.ElectricityMeterValues = nil
				if !electricityState.IsZero() || !electricityUsage.IsZero() {
					electricityMeterValues = domain.NewElectricityMeterValues().
						SetName(gem.name).
						SetElectricityState(electricityState).
						SetElectricityUsage(electricityUsage)
				}
				var gasMeterValues *domain.GasMeterValues = nil
				if !gasUsage.IsZero() {
					gasMeterValues = domain.NewGasMeterValues().
						SetName(gem.name).
						SetGasUsage(gasUsage)
				}
				var waterMeterValues *domain.WaterMeterValues = nil
				if !waterUsage.IsZero() {
					waterMeterValues = domain.NewWaterMeterValues().
						SetName(gem.name).
						SetWaterUsage(waterUsage)
				}

				gem.enrichMeterValues(electricityMeterValues, gasMeterValues, waterMeterValues)
				domain.ElectricityMeterReadings.Trigger(electricityMeterValues)
				domain.GasMeterReadings.Trigger(gasMeterValues)
				domain.WaterMeterReadings.Trigger(waterMeterValues)
			}
		}
	}()

}

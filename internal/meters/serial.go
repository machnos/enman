package meters

import (
	"bufio"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/serial"
	"time"
)

type serialMeter struct {
	serialConfig *serial.Config
	serialPort   serial.Port
	reader       *bufio.Reader
	updInterval  time.Duration
	meter        implementingEnergyMeter
}

func newSerialMeter(serialConfig *serial.Config) *serialMeter {
	return &serialMeter{
		serialConfig: serialConfig,
		updInterval:  500 * time.Millisecond,
	}
}

func (sm *serialMeter) updateInterval() time.Duration {
	return sm.updInterval
}

func (sm *serialMeter) readValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, gasUsage *domain.GasUsage, waterUsage *domain.WaterUsage) {
	sm.meter.readValues(electricityState, electricityUsage, gasUsage, waterUsage)
}

func (sm *serialMeter) shutdown() {
	sm.meter.shutdown()
	if sm.serialPort != nil {
		err := sm.serialPort.Close()
		if err != nil && log.DebugEnabled() {
			log.Debugf("Unable to close serial port: %v", err)
		}
		sm.serialPort = nil
	}
}

func (sm *serialMeter) enrichEvents(electricityMeterValues *domain.ElectricityMeterValues, gasMeterValues *domain.GasMeterValues, waterMeterValues *domain.WaterMeterValues) {
	sm.meter.enrichEvents(electricityMeterValues, gasMeterValues, waterMeterValues)
}

func probeSerialMeter(name string, _ domain.EnergySourceRole, meterConfig *config.EnergyMeter) domain.EnergyMeter {
	probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
	if meterConfig.Speed != 0 {
		probeBaudRates = []uint{uint(meterConfig.Speed)}
	}
	for _, rate := range probeBaudRates {
		serialConfig := &serial.Config{
			Address:  meterConfig.ConnectURL,
			BaudRate: int(rate),
			Timeout:  time.Second * 5,
			DataBits: 8,
			Parity:   "N",
			StopBits: 1,
		}
		if meterConfig.Brand == "DSMR" || meterConfig.Brand == "" {
			if log.InfoEnabled() {
				log.Infof("Probing for DSMR meter with baud rate %d at %s", rate, meterConfig.ConnectURL)
			}
			meter, err := newDsmrMeter(name, serialConfig, meterConfig)
			if err == nil {
				return meter
			}
			log.Infof("Probe failed for DSMR meter: %v", err)
		}
	}
	return nil
}

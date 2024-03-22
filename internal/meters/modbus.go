package meters

import (
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
	"strings"
	"time"
)

const (
	meterUsageUpdateInterval = time.Second * 10
)

type modbusMeter struct {
	modbusClient  *modbus.ModbusClient
	modbusUnitId  uint8
	usageLastRead time.Time
	updInterval   time.Duration
}

func (mm *modbusMeter) UpdateInterval() time.Duration {
	return mm.updInterval
}

func newModbusMeter(modbusClient *modbus.ModbusClient, modbusUnitId uint8) *modbusMeter {
	return &modbusMeter{
		modbusClient: modbusClient,
		modbusUnitId: modbusUnitId,
		updInterval:  500 * time.Millisecond,
	}
}

func (mm *modbusMeter) shutdown() {
	if mm.modbusClient != nil {
		// TODO a modbus client is cached (see probeModbusMeter() method), so we should only close the client if no other meter is reading from it as well.
		modbus.RemoveCached(mm.modbusClient)
		mm.modbusClient = nil
	}
}

func (mm *modbusMeter) shouldUpdateUsage() bool {
	if mm.usageLastRead.IsZero() || (time.Now().Sub(mm.usageLastRead) > meterUsageUpdateInterval) {
		mm.usageLastRead = time.Now()
		return true
	}
	return false
}

func probeModbusMeter(role domain.EnergySourceRole, meterConfig *config.EnergyMeter) domain.EnergyMeter {
	var meter domain.EnergyMeter
	if strings.HasPrefix(meterConfig.ConnectURL, "rtu") {
		probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
		if meterConfig.Speed != 0 {
			probeBaudRates = []uint{uint(meterConfig.Speed)}
		}
		for _, rate := range probeBaudRates {
			clientConfig := &modbus.ClientConfiguration{
				URL:     meterConfig.ConnectURL,
				Timeout: time.Millisecond * 500,
				Speed:   rate,
			}
			modbusClient, clientCached, err := modbus.GetOrCreateCached(clientConfig)
			if err != nil {
				if log.DebugEnabled() {
					log.Debugf("Unable to create modbus client: %v", err)
				}
				continue
			}
			meter = probeMeterWithClient(role, meterConfig, modbusClient)
			if meter != nil {
				break
			} else if !clientCached {
				modbus.RemoveCached(modbusClient)
			}
		}
	} else {
		clientConfig := &modbus.ClientConfiguration{
			URL: meterConfig.ConnectURL,
		}
		modbusClient, clientCached, err := modbus.GetOrCreateCached(clientConfig)
		if err != nil {
			if log.DebugEnabled() {
				log.Debugf("Unable to create modbus client: %v", err)
			}
		} else {
			meter = probeMeterWithClient(role, meterConfig, modbusClient)
			if meter == nil && !clientCached {
				modbus.RemoveCached(modbusClient)
			}
		}
	}
	if meter == nil {
		log.Warningf("Unable to detect modbus energy meter in role %s at url '%s'", role, meterConfig.ConnectURL)
	}
	return meter
}

func probeMeterWithClient(role domain.EnergySourceRole, meterConfig *config.EnergyMeter, modbusClient *modbus.ModbusClient) domain.EnergyMeter {
	if meterConfig.Brand == "Carlo Gavazzi" || meterConfig.Brand == "" {
		// Carlo Gavazzi meter type
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Carlo Gavazzi meter with %sunit id %d at %s", baudRateLogging, meterConfig.ModbusUnitId, modbusClient.URL())
		}
		meter, err := newCarloGavazziMeter(modbusClient, meterConfig)
		if err == nil {
			return meter
		}
		log.Infof("Probe failed for Carlo Gavazzi meter: %v", err)
	}
	if meterConfig.Brand == "ABB" || meterConfig.Brand == "" {
		// Abb meter
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for ABB meter with %sunit id %d at %s", baudRateLogging, meterConfig.ModbusUnitId, modbusClient.URL())
		}
		meter, err := newAbbMeter(modbusClient, meterConfig)
		if err == nil {
			return meter
		}
		log.Infof("Probe failed for ABB meter: %v", err)
	}
	if meterConfig.Brand == "Victron" || meterConfig.Brand == "" {
		// Victron grid meter
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Victron meter with %sunit id %d at %s", baudRateLogging, meterConfig.ModbusUnitId, modbusClient.URL())
		}
		meter, err := newVictronMeter(role, modbusClient, meterConfig)
		if err == nil {
			return meter
		}
		log.Infof("Probe failed for Victron meter: %v", err)
	}
	return nil
}

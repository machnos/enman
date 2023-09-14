package meters

import (
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	meterUsageUpdateInterval = time.Second * 10
)

var modbusClientCache = make(map[string]*modbus.ModbusClient)

type electricityModbusMeter struct {
	genericElectricityMeter
	updateInterval time.Duration
	updateTicker   *time.Ticker
	usageLastRead  time.Time
	url            string
	modbusUnitId   uint8
	modbusClient   *modbus.ModbusClient
	meterBrand     string
	meterType      string
	meterSerial    string
	probeWaitGroup sync.WaitGroup
	readValues     func(*electricityModbusMeter, *domain.ElectricityState, *domain.ElectricityUsage)
}

func NewElectricityModbusMeter(name string, role domain.ElectricitySourceRole, brand string, speed uint16, url string, modbusUnitId uint8, lineIndices []uint8, attributes string) domain.ElectricityMeter {
	em := &electricityModbusMeter{
		genericElectricityMeter: genericElectricityMeter{
			name:        name,
			role:        role,
			lineIndices: lineIndices,
			attributes:  attributes,
		},
		updateInterval: time.Millisecond * 500,
		url:            url,
		modbusUnitId:   modbusUnitId,
	}
	probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
	if speed != 0 {
		probeBaudRates = []uint{uint(speed)}
	}

	em.probeWaitGroup.Add(1)
	go em.probeMeter(probeBaudRates, brand)
	return em
}

func (emm *electricityModbusMeter) shouldUpdateUsage() bool {
	if emm.usageLastRead.IsZero() || (time.Now().Sub(emm.usageLastRead) > meterUsageUpdateInterval) {
		emm.usageLastRead = time.Now()
		return true
	}
	return false
}

func (emm *electricityModbusMeter) probeMeter(baudRates []uint, brand string) {
	defer emm.probeWaitGroup.Done()
	if strings.HasPrefix(emm.url, "rtu") {
		for _, rate := range baudRates {
			config := &modbus.ClientConfiguration{
				URL:     emm.url,
				Timeout: time.Millisecond * 500,
				Speed:   rate,
			}
			modbusClient, clientCached := modbusClientCache[emm.url]
			if !clientCached {
				client, err := emm.newModbusClient(config)
				if err != nil {
					if log.DebugEnabled() {
						log.Debugf("Unable to create modbus client: %v", err)
					}
					continue
				}
				modbusClient = client
			}

			if emm.probeMeterWithClient(modbusClient, brand) {
				modbusClientCache[emm.url] = modbusClient
				break
			} else if !clientCached {
				err := modbusClient.Close()
				if err != nil && log.DebugEnabled() {
					log.Debugf("Unable to close modbus client: %v", err)
				}
			}
		}
	} else {
		config := &modbus.ClientConfiguration{
			URL: emm.url,
		}
		modbusClient, err := emm.newModbusClient(config)
		if err != nil {
			if log.DebugEnabled() {
				log.Debugf("Unable to create modbus client: %v", err)
			}
		} else {
			if !emm.probeMeterWithClient(modbusClient, brand) {
				err := modbusClient.Close()
				if err != nil && log.DebugEnabled() {
					log.Debugf("Unable to close modbus client: %v", err)
				}
			}
		}
	}
	if emm.modbusClient == nil {
		log.Warningf("Unable to detect modbus electricity meter in role %s with name %s at url '%s'", emm.role, emm.name, emm.url)
	}
}

func (emm *electricityModbusMeter) probeMeterWithClient(modbusClient *modbus.ModbusClient, brand string) bool {
	if brand == "Carlo Gavazzi" || brand == "" {
		// Carlo Gavazzi meter type
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Carlo Gavazzi meter with %sunit id %d at %s", baudRateLogging, emm.modbusUnitId, modbusClient.URL())
		}
		cgMeterType, err := modbusClient.ReadRegister(emm.modbusUnitId, 0x000b, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err == nil {
			cg := carlo_gavazzi{}
			if cg.probe(emm, modbusClient, cgMeterType) {
				return true
			}
		}
	}
	if brand == "ABB" || brand == "" {
		// Abb meter
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for ABB meter with %sunit id %d at %s", baudRateLogging, emm.modbusUnitId, modbusClient.URL())
		}
		abbMeterType, err := modbusClient.ReadUint32(emm.modbusUnitId, 0x8960, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		if err == nil {
			abb := &abb{}
			if abb.probe(emm, modbusClient, abbMeterType) {
				return true
			}
		}
	}
	if brand == "Victron" || brand == "" {
		// Victron grid meter
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Victron meter with %sunit id %d at %s", baudRateLogging, emm.modbusUnitId, modbusClient.URL())
		}

		if domain.RoleGrid == emm.role {
			_, err := modbusClient.ReadBytes(emm.modbusUnitId, 2609, 14, modbus.INPUT_REGISTER)
			if err == nil {
				victron := &victron{}
				if victron.probe(emm, modbusClient, domain.RoleGrid) {
					return true
				}
			}
			// Victron pv meter
		} else if domain.RolePv == emm.role {
			// address 1309 gives a weird error in v3.01 of victron. A bug should be raised, because victron thinks it needs to be a battery instead of PV.
			//_, err := modbusClient.ReadBytes(emm.modbusUnitId, 1309, 14, modbus.INPUT_REGISTER)
			//if err == nil {
			victron := &victron{}
			if victron.probe(emm, modbusClient, domain.RolePv) {
				return true
			}
			//}
		}
	}
	return false
}

func (emm *electricityModbusMeter) newModbusClient(config *modbus.ClientConfiguration) (*modbus.ModbusClient, error) {
	modbusClient, err := modbus.NewClient(config)
	if err != nil {
		return nil, err
	}
	err = modbusClient.Open()
	if err != nil {
		return nil, err
	}
	return modbusClient, nil
}

func (emm *electricityModbusMeter) WaitForInitialization() bool {
	emm.probeWaitGroup.Wait()
	return emm.modbusClient != nil
}

func (emm *electricityModbusMeter) StartReading(ctx context.Context) domain.ElectricityMeter {
	if !emm.WaitForInitialization() {
		log.Warningf("Unable to read values from electricity meter %s in role %v as it is an unknown device", emm.name, emm.role)
		return emm
	}
	if emm.updateTicker != nil {
		// Meter already started
		return emm
	}
	emm.updateTicker = time.NewTicker(emm.updateInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				emm.shutdown()
				return
			case _ = <-emm.updateTicker.C:
				var electricityState *domain.ElectricityState = nil
				var electricityUsage *domain.ElectricityUsage = nil
				if emm.attributes == "state" || emm.attributes == "" {
					electricityState = domain.NewElectricityState()
				}
				if emm.shouldUpdateUsage() && (emm.attributes == "usage" || emm.attributes == "") {
					electricityUsage = domain.NewElectricityUsage()
				}
				if electricityState != nil || electricityUsage != nil {
					emm.readValues(emm, electricityState, electricityUsage)
					event := domain.NewElectricityMeterValues().
						SetName(emm.name).
						SetRole(emm.role).
						SetMeterPhases(emm.phases).
						SetMeterBrand(emm.meterBrand).
						SetMeterType(emm.meterType).
						SetMeterSerial(emm.meterSerial).
						SetReadLineIndices(emm.lineIndices).
						SetElectricityState(electricityState).
						SetElectricityUsage(electricityUsage)
					domain.ElectricityMeterReadings.Trigger(event)
				}
			}
		}
	}()
	return emm
}

func (emm *electricityModbusMeter) shutdown() {
	if emm.updateTicker != nil {
		emm.updateTicker.Stop()
		emm.updateTicker = nil
		log.Infof("Stop reading values from electricity meter %s in role %v", emm.name, emm.role)
	}
	if emm.modbusClient != nil {
		// TODO a modbus client is cached (see probe() method), so we should only close the client if no other meter is reading from it as well.
		err := emm.modbusClient.Close()
		if err != nil && log.DebugEnabled() {
			log.Debugf("Unable to close modbus client: %v", err)
		}
		delete(modbusClientCache, emm.url)
		emm.modbusClient = nil
	}
}

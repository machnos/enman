package meters_old

import (
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

type modbusEnergyMeter struct {
	updateTicker     *time.Ticker
	usageLastRead    time.Time
	url              string
	modbusUnitId     uint8
	modbusClient     *modbus.ModbusClient
	meterBrand       string
	meterType        string
	meterSerial      string
	probeWaitGroup   sync.WaitGroup
	readModbusValues func(*domain.ElectricityState, *domain.ElectricityUsage)
}

func NewModbusElectricityMeter(name string, role domain.EnergySourceRole, brand string, speed uint16, url string, modbusUnitId uint8) domain.ElectricityMeter {
	genenme := &genericEnergyMeter{
		name: name,
	}
	genelme := &genericElectricityMeter{
		role: role,
	}
	em := &modbusEnergyMeter{
		url:          url,
		modbusUnitId: modbusUnitId,
	}
	probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
	if speed != 0 {
		probeBaudRates = []uint{uint(speed)}
	}

	em.probeWaitGroup.Add(1)
	go em.probeMeter(genenme, genelme, probeBaudRates, brand)
	return nil
}

func (mem *modbusEnergyMeter) shouldUpdateUsage() bool {
	if mem.usageLastRead.IsZero() || (time.Now().Sub(mem.usageLastRead) > meterUsageUpdateInterval) {
		mem.usageLastRead = time.Now()
		return true
	}
	return false
}

func (mem *modbusEnergyMeter) probeMeter(genEnMe *genericEnergyMeter, genElMe *genericElectricityMeter, baudRates []uint, brand string) {
	defer mem.probeWaitGroup.Done()
	if strings.HasPrefix(mem.url, "rtu") {
		for _, rate := range baudRates {
			config := &modbus.ClientConfiguration{
				URL:     mem.url,
				Timeout: time.Millisecond * 500,
				Speed:   rate,
			}
			modbusClient, clientCached := modbusClientCache[mem.url]
			if !clientCached {
				client, err := mem.newModbusClient(config)
				if err != nil {
					if log.DebugEnabled() {
						log.Debugf("Unable to create modbus client: %v", err)
					}
					continue
				}
				modbusClient = client
			}

			if mem.probeMeterWithClient(genEnMe, genElMe, modbusClient, brand) {
				modbusClientCache[mem.url] = modbusClient
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
			URL: mem.url,
		}
		modbusClient, err := mem.newModbusClient(config)
		if err != nil {
			if log.DebugEnabled() {
				log.Debugf("Unable to create modbus client: %v", err)
			}
		} else {
			if !mem.probeMeterWithClient(genEnMe, genElMe, modbusClient, brand) {
				err := modbusClient.Close()
				if err != nil && log.DebugEnabled() {
					log.Debugf("Unable to close modbus client: %v", err)
				}
			}
		}
	}
	if mem.modbusClient == nil {
		log.Warningf("Unable to detect modbus electricity meter in role %s with name %s at url '%s'", genElMe.role, genEnMe.name, mem.url)
	}
}

func (mem *modbusEnergyMeter) probeMeterWithClient(genenme *genericEnergyMeter, genelme *genericElectricityMeter, modbusClient *modbus.ModbusClient, brand string) bool {
	if brand == "Carlo Gavazzi" || brand == "" {
		// Carlo Gavazzi meter type
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Carlo Gavazzi meter with %sunit id %d at %s", baudRateLogging, mem.modbusUnitId, modbusClient.URL())
		}
		cgMeterType, err := modbusClient.ReadRegister(mem.modbusUnitId, 0x000b, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err == nil {
			cg := carloGavazzi{
				genenme,
				genelme,
				mem,
			}
			if cg.probe(modbusClient, cgMeterType) {
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
			log.Infof("Probing for ABB meter with %sunit id %d at %s", baudRateLogging, mem.modbusUnitId, modbusClient.URL())
		}
		abbMeterType, err := modbusClient.ReadUint32(mem.modbusUnitId, 0x8960, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		if err == nil {
			abb := &abb{
				genenme,
				genelme,
				mem,
			}
			if abb.probe(modbusClient, abbMeterType) {
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
			log.Infof("Probing for Victron meter with %sunit id %d at %s", baudRateLogging, mem.modbusUnitId, modbusClient.URL())
		}

		if domain.RoleGrid == genelme.role {
			_, err := modbusClient.ReadBytes(mem.modbusUnitId, 2609, 14, modbus.INPUT_REGISTER)
			if err == nil {
				victron := &victron{
					genenme,
					genelme,
					mem,
				}
				if victron.probe(modbusClient, domain.RoleGrid) {
					return true
				}
			}
			// Victron pv meter
		} else if domain.RolePv == genelme.role {
			// address 1309 gives a weird error in v3.01 of victron. A bug should be raised, because victron thinks it needs to be a battery instead of PV.
			//_, err := modbusClient.ReadBytes(mem.modbusUnitId, 1309, 14, modbus.INPUT_REGISTER)
			//if err == nil {
			victron := &victron{
				genenme,
				genelme,
				mem,
			}
			if victron.probe(modbusClient, domain.RolePv) {
				return true
			}
			//}
		}
	}
	return false
}

func (mem *modbusEnergyMeter) newModbusClient(config *modbus.ClientConfiguration) (*modbus.ModbusClient, error) {
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

func (mem *modbusEnergyMeter) waitForInitialization() bool {
	mem.probeWaitGroup.Wait()
	return mem.modbusClient != nil
}

func (mem *modbusEnergyMeter) readValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, _ *domain.GasUsage, _ *domain.WaterUsage) {
	mem.readModbusValues(electricityState, electricityUsage)
}

func (mem *modbusEnergyMeter) shutdown(name string) {
	if mem.updateTicker != nil {
		mem.updateTicker.Stop()
		mem.updateTicker = nil
		log.Infof("Stop reading values from electricity meter %s in role %v", name)
	}
	if mem.modbusClient != nil {
		// TODO a modbus client is cached (see probe() method), so we should only close the client if no other meter is reading from it as well.
		err := mem.modbusClient.Close()
		if err != nil && log.DebugEnabled() {
			log.Debugf("Unable to close modbus client: %v", err)
		}
		delete(modbusClientCache, mem.url)
		mem.modbusClient = nil
	}
}

func (mem *modbusEnergyMeter) updateInterval() time.Duration {
	return time.Millisecond * 500
}

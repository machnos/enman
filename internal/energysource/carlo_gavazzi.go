package energysource

import (
	"bytes"
	"enman/internal/log"
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type carloGavazziSystem struct {
}

type carloGavazziModbusMeter struct {
	modbusUnitId uint8
	lineIndexes  []uint8
	meterCode    string
	meterType    string
	phases       uint8
	serialNumber string
	protocol     cgMeterProtocol
}

func (c *carloGavazziModbusMeter) SerialNumber() string {
	return c.SerialNumber()
}

func (c *carloGavazziModbusMeter) initialize(modbusClient *modbus.ModbusClient, modbusMeter *ModbusMeterConfig) error {
	c.modbusUnitId = modbusMeter.ModbusUnitId
	c.lineIndexes = modbusMeter.LineIndexes
	modbusClient.SetUnitId(c.modbusUnitId)
	// Read meter type
	meterType, err := modbusClient.ReadRegister(0x000B, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	c.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 71:
		c.meterType = "EM24-DIN AV"
		c.phases = 3
		c.protocol = &eM24Protocol{}
	case 72:
		c.meterType = "EM24-DIN AV5"
		c.phases = 3
		c.protocol = &eM24Protocol{}
	case 73:
		c.meterType = "EM24-DIN AV6"
		c.phases = 3
		c.protocol = &eM24Protocol{}
	case 100:
		c.meterType = "EM110-DIN AV7 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 101:
		c.meterType = "EM111-DIN AV7 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 102:
		c.meterType = "EM112-DIN AV1 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 103:
		c.meterType = "EM111-DIN AV8 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 104:
		c.meterType = "EM112-DIN AV0 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 110:
		c.meterType = "EM110-DIN AV8 1 x S1"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 114:
		c.meterType = "EM111-DIN AV5 1 X S1 X"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 120:
		c.meterType = "ET112-DIN AV0 1 x S1 X"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 121:
		c.meterType = "ET112-DIN AV1 1 x S1 X"
		c.phases = 1
		c.protocol = &ex100SeriesProtocol{}
	case 331:
		c.meterType = "EM330-DIN AV6 3"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 332:
		c.meterType = "EM330-DIN AV5 3"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 335:
		c.meterType = "ET330-DIN AV5 3"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 336:
		c.meterType = "ET330-DIN AV6 3"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 340:
		c.meterType = "EM340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 341:
		c.meterType = "EM340-DIN AV2 3 X S1"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 345:
		c.meterType = "ET340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 346:
		c.meterType = "EM341-DIN AV2 3 X OS X"
		c.phases = 3
		c.protocol = &ex300SeriesProtocol{}
	case 1744:
		c.meterType = "EM530-DIN AV5 3 X S1 X"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1745:
		c.meterType = "EM530-DIN AV5 3 X S1 PF A"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1746:
		c.meterType = "EM530-DIN AV5 3 X S1 PF B"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1747:
		c.meterType = "EM530-DIN AV5 3 X S1 PF C"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1760:
		c.meterType = "EM540-DIN AV2 3 X S1 X"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1761:
		c.meterType = "EM540-DIN AV2 3 X S1 PF A"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1762:
		c.meterType = "EM540-DIN AV2 3 X S1 PF B"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	case 1763:
		c.meterType = "EM540-DIN AV2 3 X S1 PF C"
		c.phases = 3
		c.protocol = &eM530and540Protocol{}
	default:
		c.meterType = fmt.Sprintf("Carlo Gavazzi %d", meterType)
		log.Warningf("Detected an unsupported Carlo Gavazzi meter (%d). Meter will not be queried for values.", meterType)
	}
	if c.protocol != nil {
		c.protocol.initialize(c, modbusClient)
	}
	log.Infof("Detected a %d phase Carlo Gavazzi %s (identification code %d, serial %s) with unitId %d at %s.", c.phases, c.meterType, meterType, c.serialNumber, c.modbusUnitId, modbusClient.URL())
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(em24ApplicationRegister, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", c.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(em24FrontSelectorRegister, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if frontSelector == 3 {
				log.Warning("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
					"to manually update the EM24 to 'application H', or set the front selector in an unlocked position " +
					"and reinitialize the system.")
			} else {
				err := modbusClient.WriteRegister(em24ApplicationRegister, em24ApplicationH)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type CarloGavazziConfig struct {
	ModbusUrl        string
	ModbusGridConfig *ModbusGridConfig
	ModbusPvConfigs  []*ModbusPvConfig
}

func NewCarloGavazziSystem(config *CarloGavazziConfig) (*energysource.System, error) {
	cgSystem := &carloGavazziSystem{}
	return NewModbusSystem(
		&ModbusConfig{
			ModbusUrl: config.ModbusUrl,
			Timeout:   time.Millisecond * 500,
			Speed:     9600,
		},
		config.ModbusGridConfig,
		config.ModbusPvConfigs,
		func() modbusMeter {
			return modbusMeter(&carloGavazziModbusMeter{})
		},
		cgSystem.updateValues,
	)
}

func (c *carloGavazziSystem) updateValues(client *modbus.ModbusClient, system *energysource.System) {
	pollInterval := uint16(250)
	log.Infof("Start polling Carlo Gavazzi modbus devices every %d milliseconds.", pollInterval)
	ticker := time.NewTicker(time.Millisecond * time.Duration(pollInterval))
	stopChannel := make(chan bool)
	runtime.SetFinalizer(system, func(a *energysource.System) {
		stopChannel <- true
		ticker.Stop()
	})
	defer func(client *modbus.ModbusClient) {
		_ = client.Close()
	}(client)

	var runMinute = -1
	for {
		select {
		case <-ticker.C:
			changed := false
			updateTotals := false
			_, minutes, _ := time.Now().Clock()
			if runMinute != minutes {
				runMinute = minutes
				updateTotals = true
			}
			if system.Grid() != nil {
				mGrid, ok := system.Grid().(modbusGrid)
				if ok {
					for _, meter := range mGrid.meters {
						cgMeter, ok := (*meter).(*carloGavazziModbusMeter)
						if ok && cgMeter.protocol != nil {
							if cgMeter.protocol.updateInstantValues(cgMeter, client, mGrid.EnergyFlowBase) {
								changed = true
							}
							if updateTotals {
								cgMeter.protocol.updateTotalValues(cgMeter, client, mGrid.EnergyFlowBase)
							}
						}
					}
				}
			}
			if system.Pvs() != nil {
				for _, pv := range system.Pvs() {
					mPv, ok := pv.(modbusPv)
					if ok {
						for _, meter := range mPv.meters {
							cgMeter, ok := (*meter).(*carloGavazziModbusMeter)
							if ok && cgMeter.protocol != nil && cgMeter.protocol.updateInstantValues(cgMeter, client, mPv.EnergyFlowBase) {
								changed = true
							}
							if updateTotals {
								cgMeter.protocol.updateTotalValues(cgMeter, client, mPv.EnergyFlowBase)
							}
						}
					}
				}
			}
			if changed && system.LoadUpdated() != nil {
				system.LoadUpdated() <- true
			}
		case <-stopChannel:
			return
		}
	}
}

type cgMeterProtocol interface {
	initialize(*carloGavazziModbusMeter, *modbus.ModbusClient)
	updateInstantValues(*carloGavazziModbusMeter, *modbus.ModbusClient, *energysource.EnergyFlowBase) bool
	updateTotalValues(*carloGavazziModbusMeter, *modbus.ModbusClient, *energysource.EnergyFlowBase)
}

type eM24Protocol struct {
	cgMeterProtocol
}

func (u *eM24Protocol) initialize(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	readEM24Serial(meter, modbusClient)
}

func (u *eM24Protocol) updateInstantValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *eM24Protocol) updateTotalValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x0046, 5, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 10, 0))
	}
	values, err := modbusClient.ReadRegisters(0x005c, 1, modbus.INPUT_REGISTER)
	if values != nil && err == nil {
		// No option to read provided energy per phase, so we split the energy equally over the given phases.
		provided := modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0)
		providedPerPhase := provided / float32(len(meter.lineIndexes))
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
		}
	}
}

type ex100SeriesProtocol struct {
	cgMeterProtocol
}

func (u *ex100SeriesProtocol) initialize(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *ex100SeriesProtocol) updateInstantValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziSinglePhaseMeter(meter, modbusClient, flow)
}

func (u *ex100SeriesProtocol) updateTotalValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x0010, 1, modbus.INPUT_REGISTER)
	_, _ = flow.SetEnergyConsumed(meter.lineIndexes[0], modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0))
	values, _ = modbusClient.ReadRegisters(0x0020, 1, modbus.INPUT_REGISTER)
	_, _ = flow.SetEnergyProvided(meter.lineIndexes[0], modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0))
}

type ex300SeriesProtocol struct {
	cgMeterProtocol
}

func (u *ex300SeriesProtocol) initialize(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *ex300SeriesProtocol) updateInstantValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *ex300SeriesProtocol) updateTotalValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x0040, 5, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 10, 0))
	}
	if strings.HasPrefix(meter.meterType, "ET") {
		values, _ := modbusClient.ReadRegisters(0x0060, 5, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			offset := meter.lineIndexes[ix] * 2
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 10, 0))
		}
	} else {
		values, err := modbusClient.ReadRegisters(0x004e, 1, modbus.INPUT_REGISTER)
		if values != nil && err == nil {
			// No option to read provided energy per phase, so we split the energy equally over the given phases.
			provided := modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0)
			providedPerPhase := provided / float32(len(meter.lineIndexes))
			for ix := 0; ix < len(meter.lineIndexes); ix++ {
				_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
			}
		}
	}
}

type eM530and540Protocol struct {
	cgMeterProtocol
}

func (u *eM530and540Protocol) initialize(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *eM530and540Protocol) updateInstantValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *eM530and540Protocol) updateTotalValues(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x0040, 5, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 10, 0))
	}
	values, err := modbusClient.ReadRegisters(0x004e, 1, modbus.INPUT_REGISTER)
	if values != nil && err == nil {
		// No option to read provided energy per phase, so we split the energy equally over the given phases.
		provided := modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0)
		providedPerPhase := provided / float32(len(meter.lineIndexes))
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
		}
	}
}

func updateGenericCarloGavazziThreePhaseMeter(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	changed := false
	values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadRegisters(12, 11, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		valueChanged, _ := flow.SetCurrent(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 1000, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, 6+offset, 10, 0))
		changed = changed || valueChanged
	}
	return changed
}

func updateGenericCarloGavazziSinglePhaseMeter(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	changed := false
	values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
	valueChanged, _ := flow.SetVoltage(meter.lineIndexes[0], modbusClient.ValueFromUint16ResultArray(values, 0, 10, 0))
	changed = changed || valueChanged
	valueChanged, _ = flow.SetCurrent(meter.lineIndexes[0], modbusClient.ValueFromUint16ResultArray(values, 2, 1000, 0))
	changed = changed || valueChanged
	valueChanged, _ = flow.SetPower(meter.lineIndexes[0], modbusClient.ValueFromInt16ResultArray(values, 4, 10, 0))
	changed = changed || valueChanged
	return changed
}

func readGenericCarloGavazziSerial(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x5000, 7, modbus.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

func readEM24Serial(meter *carloGavazziModbusMeter, modbusClient *modbus.ModbusClient) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x1300, 7, modbus.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

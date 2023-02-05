package carlo_gavazzi

import (
	"bytes"
	"enman/internal/energysource"
	"enman/internal/energysource/modbus"
	"enman/internal/log"
	modbusProtocol "enman/internal/modbus"
	"fmt"
	"strings"
	"time"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type Meter struct {
	modbus.Meter
	modbusUnitId uint8
	lineIndexes  []uint8
	meterCode    string
	meterType    string
	phases       uint8
	serialNumber string
	protocol     cgMeterProtocol
}

func (m *Meter) SerialNumber() string {
	return m.serialNumber
}

func (m *Meter) Initialize(modbusClient *modbusProtocol.ModbusClient, modbusMeter *modbus.MeterConfig) error {
	m.modbusUnitId = modbusMeter.ModbusUnitId
	m.lineIndexes = modbusMeter.LineIndices
	modbusClient.SetUnitId(m.modbusUnitId)
	// Read meter type
	meterType, err := modbusClient.ReadRegister(0x000b, modbusProtocol.INPUT_REGISTER)
	if err != nil {
		return err
	}
	m.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 71:
		m.meterType = "EM24-DIN AV"
		m.phases = 3
		m.protocol = &eM24Protocol{}
	case 72:
		m.meterType = "EM24-DIN AV5"
		m.phases = 3
		m.protocol = &eM24Protocol{}
	case 73:
		m.meterType = "EM24-DIN AV6"
		m.phases = 3
		m.protocol = &eM24Protocol{}
	case 100:
		m.meterType = "EM110-DIN AV7 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 101:
		m.meterType = "EM111-DIN AV7 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 102:
		m.meterType = "EM112-DIN AV1 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 103:
		m.meterType = "EM111-DIN AV8 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 104:
		m.meterType = "EM112-DIN AV0 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 110:
		m.meterType = "EM110-DIN AV8 1 x S1"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 114:
		m.meterType = "EM111-DIN AV5 1 X S1 X"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 120:
		m.meterType = "ET112-DIN AV0 1 x S1 X"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 121:
		m.meterType = "ET112-DIN AV1 1 x S1 X"
		m.phases = 1
		m.protocol = &ex100SeriesProtocol{}
	case 331:
		m.meterType = "EM330-DIN AV6 3"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 332:
		m.meterType = "EM330-DIN AV5 3"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 335:
		m.meterType = "ET330-DIN AV5 3"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 336:
		m.meterType = "ET330-DIN AV6 3"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 340:
		m.meterType = "EM340-DIN AV2 3 X S1 X"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 341:
		m.meterType = "EM340-DIN AV2 3 X S1"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 345:
		m.meterType = "ET340-DIN AV2 3 X S1 X"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 346:
		m.meterType = "EM341-DIN AV2 3 X OS X"
		m.phases = 3
		m.protocol = &ex300SeriesProtocol{}
	case 1744:
		m.meterType = "EM530-DIN AV5 3 X S1 X"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1745:
		m.meterType = "EM530-DIN AV5 3 X S1 PF A"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1746:
		m.meterType = "EM530-DIN AV5 3 X S1 PF B"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1747:
		m.meterType = "EM530-DIN AV5 3 X S1 PF C"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1760:
		m.meterType = "EM540-DIN AV2 3 X S1 X"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1761:
		m.meterType = "EM540-DIN AV2 3 X S1 PF A"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1762:
		m.meterType = "EM540-DIN AV2 3 X S1 PF B"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	case 1763:
		m.meterType = "EM540-DIN AV2 3 X S1 PF C"
		m.phases = 3
		m.protocol = &eM530and540Protocol{}
	default:
		m.meterType = fmt.Sprintf("Carlo Gavazzi %d", meterType)
		log.Warningf("Detected an unsupported Carlo Gavazzi meter (%d). Meter will not be queried for values.", meterType)
	}
	if m.protocol != nil {
		m.protocol.initialize(m, modbusClient)
	}
	log.Infof("Detected a %d phase Carlo Gavazzi %s (identification code %d, serial %s) with unitId %d at %s.", m.phases, m.meterType, meterType, m.serialNumber, m.modbusUnitId, modbusClient.URL())
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(em24ApplicationRegister, modbusProtocol.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", m.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(em24FrontSelectorRegister, modbusProtocol.INPUT_REGISTER)
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

func NewModbusConfiguration(modbusUrl string) *modbusProtocol.ClientConfiguration {
	return &modbusProtocol.ClientConfiguration{
		URL:     modbusUrl,
		Timeout: time.Millisecond * 500,
		Speed:   9600,
	}
}

func newMeter() modbus.Meter {
	return &Meter{}
}

func NewGrid(name string,
	config *modbus.GridConfig,
	updateLoop *modbus.UpdateLoop,
) (*modbus.Grid, error) {
	return modbus.NewGrid(
		name,
		config,
		newMeter,
		updateLoop,
		updateGridMeterValues,
	)
}

func updateGridMeterValues(client *modbusProtocol.ModbusClient, grid *modbus.Grid, updateKwhTotals bool, updateChannel chan energysource.Grid) {
	if grid == nil {
		return
	}
	_ = client.SetEncoding(modbusProtocol.BIG_ENDIAN, modbusProtocol.LOW_WORD_FIRST)
	changed := false
	for _, meter := range grid.Meters {
		cgMeter, ok := (*meter).(*Meter)
		if ok && cgMeter.protocol != nil {
			if cgMeter.protocol.updateInstantValues(cgMeter, client, grid.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				cgMeter.protocol.updateKwhTotalValues(cgMeter, client, grid.EnergyFlowBase)
			}
		}
	}
	if changed && updateChannel != nil {
		updateChannel <- grid
	}
}

func NewPv(name string,
	config *modbus.PvConfig,
	updateLoop *modbus.UpdateLoop,
) (*modbus.Pv, error) {
	return modbus.NewPv(
		name,
		config,
		newMeter,
		updateLoop,
		updatePvMeterValues,
	)
}

func updatePvMeterValues(client *modbusProtocol.ModbusClient, pv *modbus.Pv, updateKwhTotals bool, updateChannel chan energysource.Pv) {
	if pv == nil {
		return
	}
	_ = client.SetEncoding(modbusProtocol.BIG_ENDIAN, modbusProtocol.LOW_WORD_FIRST)
	changed := false
	for _, meter := range pv.Meters {
		cgMeter, ok := (*meter).(*Meter)
		if ok && cgMeter.protocol != nil {
			if cgMeter.protocol.updateInstantValues(cgMeter, client, pv.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				cgMeter.protocol.updateKwhTotalValues(cgMeter, client, pv.EnergyFlowBase)
			}
		}
	}
	if changed && updateChannel != nil {
		updateChannel <- pv
	}
}

type cgMeterProtocol interface {
	initialize(*Meter, *modbusProtocol.ModbusClient)
	updateInstantValues(*Meter, *modbusProtocol.ModbusClient, *energysource.EnergyFlowBase) bool
	updateKwhTotalValues(*Meter, *modbusProtocol.ModbusClient, *energysource.EnergyFlowBase)
}

type eM24Protocol struct {
	cgMeterProtocol
}

func (u *eM24Protocol) initialize(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	readEM24Serial(meter, modbusClient)
}

func (u *eM24Protocol) updateInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *eM24Protocol) updateKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadUint32s(0x0046, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint32ResultArray(values, offset, 10, 0))
	}
	values, err := modbusClient.ReadUint32s(0x005c, 1, modbusProtocol.INPUT_REGISTER)
	if values != nil && err == nil {
		// No option to read provided energy per phase, so we split the energy equally over the given phases.
		provided := modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0)
		providedPerPhase := provided / float32(len(meter.lineIndexes))
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
		}
	}
}

type ex100SeriesProtocol struct {
	cgMeterProtocol
}

func (u *ex100SeriesProtocol) initialize(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *ex100SeriesProtocol) updateInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziSinglePhaseMeter(meter, modbusClient, flow)
}

func (u *ex100SeriesProtocol) updateKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadUint32s(0x0010, 1, modbusProtocol.INPUT_REGISTER)
	_, _ = flow.SetEnergyConsumed(meter.lineIndexes[0], modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0))
	values, _ = modbusClient.ReadUint32s(0x0020, 1, modbusProtocol.INPUT_REGISTER)
	_, _ = flow.SetEnergyProvided(meter.lineIndexes[0], modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0))
}

type ex300SeriesProtocol struct {
	cgMeterProtocol
}

func (u *ex300SeriesProtocol) initialize(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *ex300SeriesProtocol) updateInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *ex300SeriesProtocol) updateKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadUint32s(0x0040, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix]
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint32ResultArray(values, offset, 10, 0))
	}
	if strings.HasPrefix(meter.meterType, "ET") {
		values, _ := modbusClient.ReadUint32s(0x0060, 3, modbusProtocol.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			offset := meter.lineIndexes[ix]
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], modbusClient.ValueFromUint32ResultArray(values, offset, 10, 0))
		}
	} else {
		values, err := modbusClient.ReadUint32s(0x004e, 1, modbusProtocol.INPUT_REGISTER)
		if values != nil && err == nil {
			// No option to read provided energy per phase, so we split the energy equally over the given phases.
			provided := modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0)
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

func (u *eM530and540Protocol) initialize(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	readGenericCarloGavazziSerial(meter, modbusClient)
}

func (u *eM530and540Protocol) updateInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	return updateGenericCarloGavazziThreePhaseMeter(meter, modbusClient, flow)
}

func (u *eM530and540Protocol) updateKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadUint32s(0x0040, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix]
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint32ResultArray(values, offset, 10, 0))
	}
	values, err := modbusClient.ReadUint32s(0x004e, 1, modbusProtocol.INPUT_REGISTER)
	if values != nil && err == nil {
		// No option to read provided energy per phase, so we split the energy equally over the given phases.
		provided := modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0)
		providedPerPhase := provided / float32(len(meter.lineIndexes))
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
		}
	}
}

func updateGenericCarloGavazziThreePhaseMeter(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	changed := false
	values, _ := modbusClient.ReadUint32s(0x0000, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix]
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint32ResultArray(values, offset, 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(0x0012, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix]
		// First set the power, because on some meters we need to flip the current sign based on power
		valueChanged, _ := flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromInt32ResultArray(values, offset, 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(0x000c, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix]
		current := modbusClient.ValueFromInt32ResultArray(values, offset, 1000, 0)
		if flow.Power(meter.lineIndexes[ix]) < 0 && current > 0 {
			current = current * -1
		}
		valueChanged, _ := flow.SetCurrent(meter.lineIndexes[ix], current)
		changed = changed || valueChanged
	}
	return changed
}

func updateGenericCarloGavazziSinglePhaseMeter(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	changed := false
	values, _ := modbusClient.ReadUint32s(0x0000, 3, modbusProtocol.INPUT_REGISTER)

	valueChanged, _ := flow.SetVoltage(meter.lineIndexes[0], modbusClient.ValueFromUint32ResultArray(values, 0, 10, 0))
	changed = changed || valueChanged
	// First set the power, because on some meters we need to flip the current sign based on power
	valueChanged, _ = flow.SetPower(meter.lineIndexes[0], modbusClient.ValueFromInt32ResultArray(values, 2, 10, 0))
	changed = changed || valueChanged

	current := modbusClient.ValueFromInt32ResultArray(values, 1, 1000, 0)
	if flow.Power(meter.lineIndexes[0]) < 0 && current > 0 {
		current = current * -1
	}
	valueChanged, _ = flow.SetCurrent(meter.lineIndexes[0], current)
	changed = changed || valueChanged
	return changed
}

func readGenericCarloGavazziSerial(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x5000, 7, modbusProtocol.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

func readEM24Serial(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(0x1300, 7, modbusProtocol.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

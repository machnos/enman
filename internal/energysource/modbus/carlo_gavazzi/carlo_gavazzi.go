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

func (meter *Meter) SerialNumber() string {
	return meter.serialNumber
}

func (meter *Meter) Initialize(modbusClient *modbusProtocol.ModbusClient, modbusMeter *modbus.MeterConfig) error {
	meter.modbusUnitId = modbusMeter.ModbusUnitId
	meter.lineIndexes = modbusMeter.LineIndices
	// Read meter type
	meterType, err := modbusClient.ReadRegister(meter.modbusUnitId, 0x000b, modbusProtocol.INPUT_REGISTER)
	if err != nil {
		return err
	}
	meter.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 71:
		meter.meterType = "EM24-DIN AV"
		meter.phases = 3
		meter.protocol = &eM24Protocol{}
	case 72:
		meter.meterType = "EM24-DIN AV5"
		meter.phases = 3
		meter.protocol = &eM24Protocol{}
	case 73:
		meter.meterType = "EM24-DIN AV6"
		meter.phases = 3
		meter.protocol = &eM24Protocol{}
	case 100:
		meter.meterType = "EM110-DIN AV7 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 101:
		meter.meterType = "EM111-DIN AV7 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 102:
		meter.meterType = "EM112-DIN AV1 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 103:
		meter.meterType = "EM111-DIN AV8 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 104:
		meter.meterType = "EM112-DIN AV0 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 110:
		meter.meterType = "EM110-DIN AV8 1 x S1"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 114:
		meter.meterType = "EM111-DIN AV5 1 X S1 X"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 120:
		meter.meterType = "ET112-DIN AV0 1 x S1 X"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 121:
		meter.meterType = "ET112-DIN AV1 1 x S1 X"
		meter.phases = 1
		meter.protocol = &ex100SeriesProtocol{}
	case 331:
		meter.meterType = "EM330-DIN AV6 3"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 332:
		meter.meterType = "EM330-DIN AV5 3"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 335:
		meter.meterType = "ET330-DIN AV5 3"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 336:
		meter.meterType = "ET330-DIN AV6 3"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 340:
		meter.meterType = "EM340-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 341:
		meter.meterType = "EM340-DIN AV2 3 X S1"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 345:
		meter.meterType = "ET340-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 346:
		meter.meterType = "EM341-DIN AV2 3 X OS X"
		meter.phases = 3
		meter.protocol = &ex300SeriesProtocol{}
	case 1744:
		meter.meterType = "EM530-DIN AV5 3 X S1 X"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1745:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF A"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1746:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF B"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1747:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF C"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1760:
		meter.meterType = "EM540-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1761:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF A"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1762:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF B"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	case 1763:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF C"
		meter.phases = 3
		meter.protocol = &eM530and540Protocol{}
	default:
		meter.meterType = fmt.Sprintf("Carlo Gavazzi %d", meterType)
		log.Warningf("Detected an unsupported Carlo Gavazzi meter (%d). Meter will not be queried for values.", meterType)
	}
	if meter.protocol != nil {
		meter.protocol.initialize(meter, modbusClient)
	}
	log.Infof("Detected a %d phase Carlo Gavazzi %s (identification code %d, serial %s) with unitId %d at %s.", meter.phases, meter.meterType, meterType, meter.serialNumber, meter.modbusUnitId, modbusClient.URL())
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(meter.modbusUnitId, em24ApplicationRegister, modbusProtocol.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", meter.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(meter.modbusUnitId, em24FrontSelectorRegister, modbusProtocol.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if frontSelector == 3 {
				log.Warning("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
					"to manually update the EM24 to 'application H', or set the front selector in an unlocked position " +
					"and reinitialize the system.")
			} else {
				err := modbusClient.WriteRegister(meter.modbusUnitId, em24ApplicationRegister, em24ApplicationH)
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
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x003e, 1, modbusProtocol.INPUT_REGISTER)
	flow.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0046, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], float64(modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0)))
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x005c, 1, modbusProtocol.INPUT_REGISTER)
	flow.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
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
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0010, 1, modbusProtocol.INPUT_REGISTER)
	_, _ = flow.SetEnergyConsumed(meter.lineIndexes[0], float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0020, 1, modbusProtocol.INPUT_REGISTER)
	_, _ = flow.SetEnergyProvided(meter.lineIndexes[0], float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
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
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0034, 1, modbusProtocol.INPUT_REGISTER)
	flow.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0040, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], float64(modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0)))
	}
	if strings.HasPrefix(meter.meterType, "ET") {
		values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0060, 3, modbusProtocol.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], float64(modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0)))
		}
	} else {
		values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x004e, 1, modbusProtocol.INPUT_REGISTER)
		flow.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
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
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0034, 1, modbusProtocol.INPUT_REGISTER)
	flow.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0040, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], float64(modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0)))
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x004e, 1, modbusProtocol.INPUT_REGISTER)
	flow.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
}

func updateGenericCarloGavazziThreePhaseMeter(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	changed := false
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0000, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0012, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		// First set the power, because on some meters we need to flip the current sign based on power
		valueChanged, _ := flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromInt32sResultArray(values, meter.lineIndexes[ix], 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x000c, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		current := modbusClient.ValueFromInt32sResultArray(values, meter.lineIndexes[ix], 1000, 0)
		if flow.Power(meter.lineIndexes[ix]) < 0 && current > 0 {
			current = current * -1
		}
		valueChanged, _ := flow.SetCurrent(meter.lineIndexes[ix], current)
		changed = changed || valueChanged
	}
	return changed
}

func updateGenericCarloGavazziSinglePhaseMeter(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	changed := false
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0000, 3, modbusProtocol.INPUT_REGISTER)

	valueChanged, _ := flow.SetVoltage(meter.lineIndexes[0], modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0))
	changed = changed || valueChanged
	// First set the power, because on some meters we need to flip the current sign based on power
	valueChanged, _ = flow.SetPower(meter.lineIndexes[0], modbusClient.ValueFromInt32sResultArray(values, 2, 10, 0))
	changed = changed || valueChanged

	current := modbusClient.ValueFromInt32sResultArray(values, 1, 1000, 0)
	if flow.Power(meter.lineIndexes[0]) < 0 && current > 0 {
		current = current * -1
	}
	valueChanged, _ = flow.SetCurrent(meter.lineIndexes[0], current)
	changed = changed || valueChanged
	return changed
}

func readGenericCarloGavazziSerial(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	values, _ := modbusClient.ReadRegisters(meter.modbusUnitId, 0x5000, 7, modbusProtocol.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

func readEM24Serial(meter *Meter, modbusClient *modbusProtocol.ModbusClient) {
	values, _ := modbusClient.ReadRegisters(meter.modbusUnitId, 0x1300, 7, modbusProtocol.INPUT_REGISTER)
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	meter.serialNumber = strings.Trim(b.String(), "0")
}

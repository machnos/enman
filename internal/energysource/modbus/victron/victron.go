package victron

import (
	"enman/internal/energysource"
	"enman/internal/energysource/modbus"
	"enman/internal/log"
	modbusProtocol "enman/internal/modbus"
	"time"
)

type Meter struct {
	modbus.Meter
	modbusUnitId uint8
	lineIndexes  []uint8
	serialNumber string
}

func (m *Meter) SerialNumber() string {
	return m.serialNumber
}

func (m *Meter) Initialize(modbusClient *modbusProtocol.ModbusClient, modbusMeter *modbus.MeterConfig) error {
	m.modbusUnitId = modbusMeter.ModbusUnitId
	m.lineIndexes = modbusMeter.LineIndices
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
	changed := false
	for _, meter := range grid.Meters {
		vMeter, ok := (*meter).(*Meter)
		if ok {
			if updateGridValues(vMeter, client, grid.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				updateGridTotals(vMeter, client, grid.EnergyFlowBase)
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
	changed := false
	for _, meter := range pv.Meters {
		vMeter, ok := (*meter).(*Meter)
		if ok {
			if updatePvValues(vMeter, client, pv.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				updatePvTotals(vMeter, client, pv.EnergyFlowBase)
			}
		}
	}
	if changed && updateChannel != nil {
		updateChannel <- pv
	}
}

func updateGridValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	if len(meter.serialNumber) == 0 {
		// Cannot set the serial number in the initialize function because we don't know the role (pv,grid etc) of the meter over there.
		// Unfortunately different meter roles have different addresses to read the serial number of the meter.
		bytes, err := modbusClient.ReadBytes(2609, 14, modbusProtocol.INPUT_REGISTER)
		if err != nil {
			log.Warningf("Unable to read Victron serial: %s", err.Error())
			meter.serialNumber = "unknown"
		} else {
			meter.serialNumber = string(bytes)
		}
	}

	changed := false
	values, _ := modbusClient.ReadRegisters(2600, 3, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		valueChanged, _ := flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, meter.lineIndexes[ix], 0, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadRegisters(2616, 6, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+0, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetCurrent(meter.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, offset+1, 10, 0))
		changed = changed || valueChanged
	}
	return changed
}

func updatePvValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(meter.modbusUnitId)
	if len(meter.serialNumber) == 0 {
		// Cannot set the serial number in the initialize function because we don't know the role (pv,grid etc) of the meter over there.
		// Unfortunately different meter roles have different addresses to read the serial number of the meter.
		bytes, err := modbusClient.ReadBytes(1039, 14, modbusProtocol.INPUT_REGISTER)
		if err != nil {
			log.Warningf("Unable to read Victron serial: %s", err.Error())
			meter.serialNumber = "unknown"
		} else {
			meter.serialNumber = string(bytes)
		}
	}
	changed := false
	values, _ := modbusClient.ReadRegisters(1027, 11, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 4
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+0, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetCurrent(meter.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, offset+1, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+2, 0, 0))
		changed = changed || valueChanged
	}
	return changed
}

func updateGridTotals(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(2623, 11, modbusProtocol.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 100, 0))
	}
	values, err := modbusClient.ReadRegisters(2637, 1, modbusProtocol.INPUT_REGISTER)
	if values != nil && err == nil {
		// Provided energy per phase is far from correct, so we split the total energy (which seems to be correct) equally over the given phases.
		provided := modbusClient.ValueFromUint16ResultArray(values, 0, 100, 0)
		providedPerPhase := provided / float32(len(meter.lineIndexes))
		for ix := 0; ix < len(meter.lineIndexes); ix++ {
			_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], providedPerPhase)
		}
	}
}

func updatePvTotals(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(meter.modbusUnitId)
	values, err := modbusClient.ReadRegisters(1046, 5, modbusProtocol.INPUT_REGISTER)
	if err != nil || values == nil || len(values) < 3 {
		return
	}
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		offset := meter.lineIndexes[ix] * 2
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset, 100, 0))
	}
}

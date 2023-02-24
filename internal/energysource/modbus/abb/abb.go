package abb

import (
	"enman/internal/energysource"
	"enman/internal/energysource/modbus"
	"enman/internal/log"
	modbusProtocol "enman/internal/modbus"
	"fmt"
	"time"
)

type Meter struct {
	modbus.Meter
	modbusUnitId uint8
	lineIndexes  []uint8
	meterCode    string
	meterType    string
	phases       uint8
	serialNumber string
}

func (meter *Meter) SerialNumber() string {
	return meter.serialNumber
}

func (meter *Meter) Initialize(modbusClient *modbusProtocol.ModbusClient, modbusMeter *modbus.MeterConfig) error {
	meter.modbusUnitId = modbusMeter.ModbusUnitId
	meter.lineIndexes = modbusMeter.LineIndices
	// Read meter type
	meterType, err := modbusClient.ReadUint32(meter.modbusUnitId, 0x8960, modbusProtocol.HOLDING_REGISTER)
	if err != nil {
		return err
	}
	meter.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 0x42323120:
		meter.meterType = "ABB B21"
		meter.phases = 1
	case 0x42323320:
		meter.meterType = "ABB B23"
		meter.phases = 3
	case 0x42323420:
		meter.meterType = "ABB B24"
		meter.phases = 3
	default:
		meter.meterType = fmt.Sprintf("ABB %d", meterType)
		log.Warningf("Detected an unsupported ABB meter (%d). Meter will not be queried for values.", meterType)
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

func updateGridMeterValues(modbusClient *modbusProtocol.ModbusClient, grid *modbus.Grid, updateKwhTotals bool, updateChannel chan energysource.Grid) {
	if grid == nil {
		return
	}
	_ = modbusClient.SetEncoding(modbusProtocol.BIG_ENDIAN, modbusProtocol.HIGH_WORD_FIRST)
	changed := false
	for _, meter := range grid.Meters {
		abbMeter, ok := (*meter).(*Meter)
		if ok {
			if updateInstantValues(abbMeter, modbusClient, grid.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				updateKwhTotalValues(abbMeter, modbusClient, grid.EnergyFlowBase)
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

func updatePvMeterValues(modbusClient *modbusProtocol.ModbusClient, pv *modbus.Pv, updateKwhTotals bool, updateChannel chan energysource.Pv) {
	if pv == nil {
		return
	}
	_ = modbusClient.SetEncoding(modbusProtocol.BIG_ENDIAN, modbusProtocol.HIGH_WORD_FIRST)
	changed := false
	for _, meter := range pv.Meters {
		abbMeter, ok := (*meter).(*Meter)
		if ok {
			if updateInstantValues(abbMeter, modbusClient, pv.EnergyFlowBase) {
				changed = true
			}
			if updateKwhTotals {
				updateKwhTotalValues(abbMeter, modbusClient, pv.EnergyFlowBase)
			}
		}
	}
	if changed && updateChannel != nil {
		updateChannel <- pv
	}
}

func updateInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	changed := false
	if meter.phases == 1 {
		changed = updateSinglePhaseMeterInstantValues(meter, modbusClient, flow)
	} else if meter.phases == 3 {
		changed = updateThreePhaseMeterInstantValues(meter, modbusClient, flow)
	}
	return changed
}

func updateSinglePhaseMeterInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	changed := false
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b00, 1, modbusProtocol.HOLDING_REGISTER)
	valueChanged, _ := flow.SetVoltage(meter.lineIndexes[0], modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0))
	changed = changed || valueChanged

	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b0c, 1, modbusProtocol.HOLDING_REGISTER)
	valueChanged, _ = flow.SetCurrent(meter.lineIndexes[0], modbusClient.ValueFromUint32sResultArray(values, 0, 100, 0))
	changed = changed || valueChanged

	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b14, 1, modbusProtocol.HOLDING_REGISTER)
	valueChanged, _ = flow.SetPower(meter.lineIndexes[0], modbusClient.ValueFromInt32sResultArray(values, 0, 100, 0))
	changed = changed || valueChanged
	return changed
}

func updateThreePhaseMeterInstantValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	changed := false
	values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b00, 3, modbusProtocol.HOLDING_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		valueChanged, _ := flow.SetVoltage(meter.lineIndexes[ix], modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 10, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b0c, 3, modbusProtocol.HOLDING_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		valueChanged, _ := flow.SetCurrent(meter.lineIndexes[ix], modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 100, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b16, 3, modbusProtocol.HOLDING_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		valueChanged, _ := flow.SetPower(meter.lineIndexes[ix], modbusClient.ValueFromUint32sResultArray(values, meter.lineIndexes[ix], 100, 0))
		changed = changed || valueChanged
	}
	return changed
}

func updateKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	if meter.phases == 1 {
		updateSinglePhaseKwhTotalValues(meter, modbusClient, flow)
	} else if meter.phases == 3 {
		updateThreePhaseKwhTotalValues(meter, modbusClient, flow)
	}
}

func updateSinglePhaseKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	values, _ := modbusClient.ReadUint64s(meter.modbusUnitId, 0x5000, 2, modbusProtocol.HOLDING_REGISTER)
	_, _ = flow.SetEnergyConsumed(meter.lineIndexes[0], modbusClient.ValueFromUint64sResultArray(values, 0, 100, 0))
	_, _ = flow.SetEnergyProvided(meter.lineIndexes[0], modbusClient.ValueFromUint64sResultArray(values, 1, 100, 0))
}

func updateThreePhaseKwhTotalValues(meter *Meter, modbusClient *modbusProtocol.ModbusClient, flow *energysource.EnergyFlowBase) {
	values, _ := modbusClient.ReadUint64s(meter.modbusUnitId, 0x5000, 2, modbusProtocol.HOLDING_REGISTER)
	flow.SetTotalEnergyConsumed(modbusClient.ValueFromUint64sResultArray(values, 0, 100, 0))
	flow.SetTotalEnergyProvided(modbusClient.ValueFromUint64sResultArray(values, 1, 100, 0))

	values, _ = modbusClient.ReadUint64s(meter.modbusUnitId, 0x5460, 3, modbusProtocol.HOLDING_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		_, _ = flow.SetEnergyConsumed(meter.lineIndexes[ix], modbusClient.ValueFromUint64sResultArray(values, meter.lineIndexes[ix], 100, 0))
	}
	values, _ = modbusClient.ReadUint64s(meter.modbusUnitId, 0x546c, 3, modbusProtocol.HOLDING_REGISTER)
	for ix := 0; ix < len(meter.lineIndexes); ix++ {
		_, _ = flow.SetEnergyProvided(meter.lineIndexes[ix], modbusClient.ValueFromUint64sResultArray(values, meter.lineIndexes[ix], 100, 0))
	}
}

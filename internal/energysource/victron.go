package energysource

import (
	"enman/internal/log"
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"runtime"
	"time"
)

type victronSystem struct {
}

type victronModbusMeter struct {
	modbusUnitId uint8
	lineIndexes  []uint8
	serialNumber string
}

func (v *victronModbusMeter) SerialNumber() string {
	return v.SerialNumber()
}

func (v *victronModbusMeter) initialize(modbusClient *modbus.ModbusClient, modbusMeter *ModbusMeterConfig) error {
	v.modbusUnitId = modbusMeter.ModbusUnitId
	v.lineIndexes = modbusMeter.LineIndexes
	return nil
}

type VictronConfig struct {
	ModbusUrl        string
	ModbusGridConfig *ModbusGridConfig
	ModbusPvConfigs  []*ModbusPvConfig
}

func NewVictronSystem(config *VictronConfig) (*energysource.System, error) {
	vSystem := &victronSystem{}
	return NewModbusSystem(
		&ModbusConfig{
			ModbusUrl: config.ModbusUrl,
			Timeout:   time.Millisecond * 500,
		},
		config.ModbusGridConfig,
		config.ModbusPvConfigs,
		func() modbusMeter {
			return modbusMeter(&victronModbusMeter{})
		},
		vSystem.readSystemValues,
	)
}

func (c *victronSystem) readSystemValues(client *modbus.ModbusClient, system *energysource.System) {
	pollInterval := uint16(250)
	log.Infof("Start polling Victron modbus devices every %d milliseconds.", pollInterval)
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
						vMeter, ok := (*meter).(*victronModbusMeter)
						if ok {
							if vMeter.updateGridValues(client, mGrid.EnergyFlowBase) {
								changed = true
							}
							if updateTotals {
								vMeter.updateGridTotals(client, mGrid.EnergyFlowBase)
							}
						}
					}
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					mPV, ok := system.Pvs()[ix].(modbusPv)
					if ok {
						for _, meter := range mPV.meters {
							vMeter, ok := (*meter).(*victronModbusMeter)
							if ok {
								if vMeter.updatePvValues(client, mPV.EnergyFlowBase) {
									changed = true
								}
								if updateTotals {
									vMeter.updatePvTotals(client, mPV.EnergyFlowBase)
								}
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

func (v *victronModbusMeter) updateGridValues(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(v.modbusUnitId)
	if len(v.serialNumber) == 0 {
		// Cannot set the serial number in the initialize function because we don't know the role (pv,grid etc) of the meter over there.
		// Unfortunately different meter roles have different addresses to read the serial number of the meter.
		bytes, err := modbusClient.ReadBytes(2609, 14, modbus.INPUT_REGISTER)
		if err != nil {
			log.Warningf("Unable to read Victron serial: %s", err.Error())
			v.serialNumber = "unknown"
		} else {
			v.serialNumber = string(bytes)
		}
	}

	changed := false
	values, _ := modbusClient.ReadRegisters(2600, 3, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		valueChanged, _ := flow.SetPower(v.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, v.lineIndexes[ix], 0, 0))
		changed = changed || valueChanged
	}
	values, _ = modbusClient.ReadRegisters(2616, 6, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		offset := v.lineIndexes[ix] * 2
		valueChanged, _ := flow.SetVoltage(v.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+0, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetCurrent(v.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, offset+1, 10, 0))
		changed = changed || valueChanged
	}
	return changed
}

func (v *victronModbusMeter) updatePvValues(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) bool {
	modbusClient.SetUnitId(v.modbusUnitId)
	if len(v.serialNumber) == 0 {
		// Cannot set the serial number in the initialize function because we don't know the role (pv,grid etc) of the meter over there.
		// Unfortunately different meter roles have different addresses to read the serial number of the meter.
		bytes, err := modbusClient.ReadBytes(1039, 14, modbus.INPUT_REGISTER)
		if err != nil {
			log.Warningf("Unable to read Victron serial: %s", err.Error())
			v.serialNumber = "unknown"
		} else {
			v.serialNumber = string(bytes)
		}
	}
	changed := false
	values, _ := modbusClient.ReadRegisters(1027, 11, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		offset := v.lineIndexes[ix] * 4
		valueChanged, _ := flow.SetVoltage(v.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+0, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetCurrent(v.lineIndexes[ix], modbusClient.ValueFromInt16ResultArray(values, offset+1, 10, 0))
		changed = changed || valueChanged
		valueChanged, _ = flow.SetPower(v.lineIndexes[ix], modbusClient.ValueFromUint16ResultArray(values, offset+2, 0, 0))
		changed = changed || valueChanged
	}
	return changed
}

func (v *victronModbusMeter) updateGridTotals(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(v.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(2635, 3, modbus.INPUT_REGISTER)
	flow.SetTotalEnergyConsumed(modbusClient.ValueFromUint16ResultArray(values, 0, 100, 0))
	flow.SetTotalEnergyProvided(modbusClient.ValueFromUint16ResultArray(values, 2, 100, 0))
}

func (v *victronModbusMeter) updatePvTotals(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(v.modbusUnitId)
	values, err := modbusClient.ReadRegisters(1046, 5, modbus.INPUT_REGISTER)
	if err != nil || values == nil || len(values) < 3 {
		return
	}

	total := modbusClient.ValueFromUint16ResultArray(values, 0, 100, 0) +
		modbusClient.ValueFromUint16ResultArray(values, 2, 100, 0) +
		modbusClient.ValueFromUint16ResultArray(values, 4, 100, 0)
	flow.SetTotalEnergyConsumed(total)
}

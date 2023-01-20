package energysource

import (
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
	ticker := time.NewTicker(time.Millisecond * 250)
	tickerChannel := make(chan bool)
	runtime.SetFinalizer(system, func(a *energysource.System) {
		tickerChannel <- true
		ticker.Stop()
	})
	defer func(client *modbus.ModbusClient) {
		_ = client.Close()
	}(client)

	for {
		select {
		case <-ticker.C:
			if system.Grid() != nil {
				mGrid, ok := (*system.Grid()).(modbusGrid)
				if ok {
					for _, meter := range mGrid.meters {
						vMeter, ok := (*meter).(*victronModbusMeter)
						if ok {
							vMeter.updateGridValues(client, mGrid.EnergyFlowBase)
						}
					}
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					mPV, ok := (*system.Pvs()[ix]).(modbusPv)
					if ok {
						for _, meter := range mPV.meters {
							vMeter, ok := (*meter).(*victronModbusMeter)
							if ok {
								vMeter.updatePvValues(client, mPV.EnergyFlowBase)
							}
						}
					}
				}
			}
		case <-tickerChannel:
			return
		}
	}
}

func (v *victronModbusMeter) updateGridValues(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(v.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(2600, 3, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		_ = flow.SetPower(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, v.lineIndexes[ix], 0, 0))
	}
	values, _ = modbusClient.ReadRegisters(2616, 6, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		offset := v.lineIndexes[ix] * 2
		_ = flow.SetVoltage(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset+0, 10, 0))
		_ = flow.SetCurrent(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset+1, 10, 0))
	}
}

func (v *victronModbusMeter) updatePvValues(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(v.modbusUnitId)
	values, _ := modbusClient.ReadRegisters(1027, 11, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(v.lineIndexes); ix++ {
		offset := v.lineIndexes[ix] * 4
		_ = flow.SetVoltage(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset+0, 10, 0))
		_ = flow.SetCurrent(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset+1, 10, 0))
		_ = flow.SetPower(v.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset+2, 0, 0))
	}
}

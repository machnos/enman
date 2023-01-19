package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"runtime"
	"time"
)

type victronSystem struct {
}

type victronGrid struct {
	*energysource.GridBase
	meter *victronModbusMeter
}

type victronPv struct {
	*energysource.PvBase
	meter *victronModbusMeter
}

type victronModbusMeter struct {
	modbusUnitId uint8
	lineIndexes  []uint8
}

func (v *victronModbusMeter) initialize(modbusMeter *ModbusMeter) {
	v.modbusUnitId = modbusMeter.ModbusUnitId
	v.lineIndexes = modbusMeter.LineIndexes
}

type VictronConfig struct {
	ModbusUrl        string
	ModbusGridConfig *ModbusGridConfig
	ModbusPvConfigs  []*ModbusPvConfig
}

func NewVictronSystem(config *VictronConfig) (*energysource.System, error) {
	modbusConfig := &modbus.ClientConfiguration{
		URL:     config.ModbusUrl,
		Timeout: time.Millisecond * 500,
	}
	modbusClient, err := modbus.NewClient(modbusConfig)
	if err != nil {
		return nil, err
	}
	err = modbusClient.Open()
	if err != nil {
		return nil, err
	}
	var grid *energysource.Grid = nil
	if config.ModbusGridConfig != nil {
		meter := &victronModbusMeter{}
		meter.initialize(config.ModbusGridConfig.ModbusMeter)
		g := energysource.Grid(victronGrid{
			GridBase: energysource.NewGridBase(config.ModbusGridConfig.GridConfig),
			meter:    meter,
		})
		grid = &g
	}
	var pvs []*energysource.Pv = nil
	if config.ModbusPvConfigs != nil {
		for ix := 0; ix < len(config.ModbusPvConfigs); ix++ {
			meter := &victronModbusMeter{}
			meter.initialize(config.ModbusPvConfigs[ix].ModbusMeter)
			pv := energysource.Pv(victronPv{
				PvBase: energysource.NewPvBase(config.ModbusPvConfigs[ix].PvConfig),
				meter:  meter,
			})
			pvs = append(pvs, &pv)
		}
	}
	system := energysource.NewSystem(grid, pvs)
	vSystem := &victronSystem{}
	go vSystem.readSystemValues(modbusClient, system)
	return system, nil
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
				vGrid, ok := (*system.Grid()).(victronGrid)
				if ok {
					vGrid.meter.updateGridValues(client, vGrid.EnergyFlowBase)
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					vPv, ok := (*system.Pvs()[ix]).(victronPv)
					if ok {
						vPv.meter.updatePvValues(client, vPv.EnergyFlowBase)
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

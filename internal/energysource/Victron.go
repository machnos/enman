package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
)

func NewVictronSystem(modbusUrl string, gridConfig *energysource.GridConfig, gridUnitId *uint8, pvUnitIds []uint8) (*energysource.System, error) {
	config := &ModbusConfig{
		modbusUrl:  modbusUrl,
		gridConfig: gridConfig,
		updateGridValues: func(modbusClient *modbus.ModbusClient, grid *modbusGrid) {
			if grid.modbusUnitId <= 0 {
				return
			}
			modbusClient.SetUnitId(grid.modbusUnitId)
			values, _ := modbusClient.ReadRegisters(2600, 3, modbus.INPUT_REGISTER)
			_ = grid.SetPower(0, modbusClient.ValueFromResultArray(values, 0, 0, 0))
			_ = grid.SetPower(1, modbusClient.ValueFromResultArray(values, 1, 0, 0))
			_ = grid.SetPower(2, modbusClient.ValueFromResultArray(values, 2, 0, 0))
			values, _ = modbusClient.ReadRegisters(2616, 6, modbus.INPUT_REGISTER)
			_ = grid.SetVoltage(0, modbusClient.ValueFromResultArray(values, 0, 10, 0))
			_ = grid.SetCurrent(0, modbusClient.ValueFromResultArray(values, 1, 10, 0))
			_ = grid.SetVoltage(1, modbusClient.ValueFromResultArray(values, 2, 10, 0))
			_ = grid.SetCurrent(1, modbusClient.ValueFromResultArray(values, 3, 10, 0))
			_ = grid.SetVoltage(2, modbusClient.ValueFromResultArray(values, 4, 10, 0))
			_ = grid.SetCurrent(2, modbusClient.ValueFromResultArray(values, 5, 10, 0))
		},
		updatePvValues: func(client *modbus.ModbusClient, pv *modbusPv) {
			if pv.modbusUnitId <= 0 {
				return
			}
			client.SetUnitId(pv.modbusUnitId)
			values, _ := client.ReadRegisters(1027, 11, modbus.INPUT_REGISTER)
			_ = pv.SetVoltage(0, client.ValueFromResultArray(values, 0, 10, 0))
			_ = pv.SetCurrent(0, client.ValueFromResultArray(values, 1, 10, 0))
			_ = pv.SetPower(0, client.ValueFromResultArray(values, 2, 0, 0))
			_ = pv.SetVoltage(1, client.ValueFromResultArray(values, 4, 10, 0))
			_ = pv.SetCurrent(1, client.ValueFromResultArray(values, 5, 10, 0))
			_ = pv.SetPower(1, client.ValueFromResultArray(values, 6, 0, 0))
			_ = pv.SetVoltage(2, client.ValueFromResultArray(values, 8, 10, 0))
			_ = pv.SetCurrent(2, client.ValueFromResultArray(values, 9, 10, 0))
			_ = pv.SetPower(2, client.ValueFromResultArray(values, 10, 0, 0))
		},
	}
	if gridUnitId != nil {
		config.modbusGridConfig = &ModbusGridConfig{
			modbusUnitId: *gridUnitId,
		}
	}
	if pvUnitIds != nil {
		configs := make([]*ModbusPvConfig, len(pvUnitIds))
		for ix := 0; ix < len(pvUnitIds); ix++ {
			configs = append(configs, &ModbusPvConfig{
				modbusUnitId: pvUnitIds[ix],
			})
		}
		config.pvConfigs = configs
	}
	system, err := NewModbusSystem(config)
	return system, err
}

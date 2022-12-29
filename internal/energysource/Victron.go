package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
)

func NewVictronSystem(modbusUrl string, gridConfig *energysource.GridConfig, gridUnitId *uint8, pvUnitIds []uint8) (*energysource.System, error) {
	config := &ModbusConfig{
		modbusUrl:  modbusUrl,
		gridConfig: gridConfig,
		updateGridValues: func(client *modbus.ModbusClient, grid *modbusGrid) {
			if grid.modbusUnitId <= 0 {
				return
			}
			client.SetUnitId(grid.modbusUnitId)
			values, _ := client.ReadRegisters(2600, 3, modbus.INPUT_REGISTER)
			_ = grid.SetPower(0, getValueFromRegisterResultArray(values, 0, 0, 0))
			_ = grid.SetPower(1, getValueFromRegisterResultArray(values, 1, 0, 0))
			_ = grid.SetPower(2, getValueFromRegisterResultArray(values, 2, 0, 0))
			values, _ = client.ReadRegisters(2616, 6, modbus.INPUT_REGISTER)
			_ = grid.SetVoltage(0, getValueFromRegisterResultArray(values, 0, 10, 0))
			_ = grid.SetCurrent(0, getValueFromRegisterResultArray(values, 1, 10, 0))
			_ = grid.SetVoltage(1, getValueFromRegisterResultArray(values, 2, 10, 0))
			_ = grid.SetCurrent(1, getValueFromRegisterResultArray(values, 3, 10, 0))
			_ = grid.SetVoltage(2, getValueFromRegisterResultArray(values, 4, 10, 0))
			_ = grid.SetCurrent(2, getValueFromRegisterResultArray(values, 5, 10, 0))
		},
		updatePvValues: func(client *modbus.ModbusClient, pv *modbusPv) {
			if pv.modbusUnitId <= 0 {
				return
			}
			client.SetUnitId(pv.modbusUnitId)
			values, _ := client.ReadRegisters(1027, 11, modbus.INPUT_REGISTER)
			_ = pv.SetVoltage(0, getValueFromRegisterResultArray(values, 0, 10, 0))
			_ = pv.SetCurrent(0, getValueFromRegisterResultArray(values, 1, 10, 0))
			_ = pv.SetPower(0, getValueFromRegisterResultArray(values, 2, 0, 0))
			_ = pv.SetVoltage(1, getValueFromRegisterResultArray(values, 4, 10, 0))
			_ = pv.SetCurrent(1, getValueFromRegisterResultArray(values, 5, 10, 0))
			_ = pv.SetPower(1, getValueFromRegisterResultArray(values, 6, 0, 0))
			_ = pv.SetVoltage(2, getValueFromRegisterResultArray(values, 8, 10, 0))
			_ = pv.SetCurrent(2, getValueFromRegisterResultArray(values, 9, 10, 0))
			_ = pv.SetPower(2, getValueFromRegisterResultArray(values, 10, 0, 0))

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

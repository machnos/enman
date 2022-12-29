package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"fmt"
	"strconv"
	"time"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type carloGavazziMeter struct {
	meterCode string
	meterType string
}

func NewCarloGavazziSystem(modbusUrl string, gridConfig *energysource.GridConfig, gridUnitId *uint8, pvUnitIds []uint8) (*energysource.System, error) {
	config := &ModbusConfig{
		modbusUrl:   modbusUrl,
		modbusSpeed: 9600,
		timeout:     time.Millisecond * 500,
		gridConfig:  gridConfig,
		updateGridValues: func(client *modbus.ModbusClient, grid *modbusGrid) {
			if grid.modbusUnitId <= 0 {
				return
			}
			c := &carloGavazziMeter{
				meterCode: grid.meterCode,
			}
			c.updateValues(client, grid.modbusUnitId, grid.EnergyFlowBase)
		},
		updatePvValues: func(client *modbus.ModbusClient, pv *modbusPv) {
			if pv.modbusUnitId <= 0 {
				return
			}
			c := &carloGavazziMeter{
				meterCode: pv.meterCode,
			}
			c.updateValues(client, pv.modbusUnitId, pv.EnergyFlowBase)
		},
	}
	if gridUnitId != nil {
		config.modbusGridConfig = &ModbusGridConfig{
			modbusUnitId: *gridUnitId,
			initialize: func(modbusClient *modbus.ModbusClient, grid *modbusGrid) error {
				c := &carloGavazziMeter{}
				err := c.initialize(modbusClient, grid.modbusUnitId)
				if err != nil {
					return err
				}
				grid.meterCode = c.meterCode
				grid.meterType = c.meterType
				return nil
			},
		}
	}
	if pvUnitIds != nil {
		configs := make([]*ModbusPvConfig, len(pvUnitIds))
		for ix := 0; ix < len(pvUnitIds); ix++ {
			configs = append(configs, &ModbusPvConfig{
				modbusUnitId: pvUnitIds[ix],
				initialize: func(modbusClient *modbus.ModbusClient, pv *modbusPv) error {
					c := &carloGavazziMeter{}
					err := c.initialize(modbusClient, pv.modbusUnitId)
					if err != nil {
						return err
					}
					pv.meterCode = c.meterCode
					pv.meterType = c.meterType
					return nil
				},
			})
		}
		config.pvConfigs = configs
	}
	system, err := NewModbusSystem(config)
	return system, err
}

func (c *carloGavazziMeter) initialize(modbusClient *modbus.ModbusClient, modbusUnitId uint8) error {
	modbusClient.SetUnitId(modbusUnitId)
	meterType, err := modbusClient.ReadRegister(0x000B, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	c.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 71:
		c.meterType = "EM24-DIN AV"
	case 72:
		c.meterType = "EM24-DIN AV5"
	case 73:
		c.meterType = "EM24-DIN AV6"
	case 100:
		c.meterType = "EM110-DIN AV7 1 x S1"
	case 101:
		c.meterType = "EM111-DIN AV7 1 x S1"
	case 102:
		c.meterType = "EM112-DIN AV1 1 x S1"
	case 103:
		c.meterType = "EM111-DIN AV8 1 x S1"
	case 104:
		c.meterType = "EM112-DIN AV0 1 x S1"
	case 110:
		c.meterType = "EM110-DIN AV8 1 x S1"
	case 114:
		c.meterType = "EM111-DIN AV5 1 X S1 X"
	case 120:
		c.meterType = "ET112-DIN AV0 1 x S1 X"
	case 121:
		c.meterType = "ET112-DIN AV1 1 x S1 X"
	case 331:
		c.meterType = "EM330-DIN AV6 3"
	case 332:
		c.meterType = "EM330-DIN AV5 3"
	case 335:
		c.meterType = "ET330-DIN AV5 3"
	case 336:
		c.meterType = "ET330-DIN AV6 3"
	case 340:
		c.meterType = "EM340-DIN AV2 3 X S1 X"
	case 341:
		c.meterType = "EM340-DIN AV2 3 X S1"
	case 345:
		c.meterType = "ET340-DIN AV2 3 X S1 X"
	case 346:
		c.meterType = "EM341-DIN AV2 3 X OS X"
	case 1744:
		c.meterType = "EM530-DIN AV5 3 X S1 X"
	case 1745:
		c.meterType = "EM530-DIN AV5 3 X S1 PF A"
	case 1746:
		c.meterType = "EM530-DIN AV5 3 X S1 PF B"
	case 1747:
		c.meterType = "EM530-DIN AV5 3 X S1 PF C"
	case 1760:
		c.meterType = "EM540-DIN AV2 3 X S1 X"
	case 1761:
		c.meterType = "EM540-DIN AV2 3 X S1 PF A"
	case 1762:
		c.meterType = "EM540-DIN AV2 3 X S1 PF B"
	case 1763:
		c.meterType = "EM540-DIN AV2 3 X S1 PF C"
	default:
		c.meterType = fmt.Sprintf("Carlo Gavazzo %d", meterType)
	}
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(em24ApplicationRegister, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(em24FrontSelectorRegister, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if frontSelector == 3 {
				println("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
					"to manually update the EM24 to 'applicatin H', or set the front selector in an unlocked position " +
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

func (c *carloGavazziMeter) threePhase() bool {
	code, _ := strconv.Atoi(c.meterCode)
	switch code {
	case 71, 72, 73, 331, 332, 335, 336, 340, 341, 345, 346, 1744, 1745, 1746, 1747, 1760, 1761, 1762, 1763:
		return true
	}
	return false
}

func (c *carloGavazziMeter) updateValues(modbusClient *modbus.ModbusClient, modbusUnitId uint8, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(modbusUnitId)
	if c.threePhase() {
		values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
		_ = flow.SetVoltage(0, getValueFromRegisterResultArray(values, 0, 10, 0))
		_ = flow.SetVoltage(1, getValueFromRegisterResultArray(values, 2, 10, 0))
		_ = flow.SetVoltage(2, getValueFromRegisterResultArray(values, 4, 10, 0))
		values, _ = modbusClient.ReadRegisters(12, 11, modbus.INPUT_REGISTER)
		_ = flow.SetCurrent(0, getValueFromRegisterResultArray(values, 0, 1000, 0))
		_ = flow.SetCurrent(1, getValueFromRegisterResultArray(values, 2, 1000, 0))
		_ = flow.SetCurrent(2, getValueFromRegisterResultArray(values, 4, 1000, 0))
		_ = flow.SetPower(0, getValueFromRegisterResultArray(values, 6, 10, 0))
		_ = flow.SetPower(1, getValueFromRegisterResultArray(values, 8, 10, 0))
		_ = flow.SetPower(2, getValueFromRegisterResultArray(values, 10, 10, 0))
	} else {
		values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
		_ = flow.SetVoltage(0, getValueFromRegisterResultArray(values, 0, 10, 0))
		_ = flow.SetCurrent(0, getValueFromRegisterResultArray(values, 2, 1000, 0))
		_ = flow.SetPower(0, getValueFromRegisterResultArray(values, 4, 10, 0))
	}
}

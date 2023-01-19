package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"fmt"
	"runtime"
	"time"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type carloGavazziSystem struct {
}

type carloGavazziGrid struct {
	*energysource.GridBase
	meter *carloGavazziModbusMeter
}

type carloGavazziPv struct {
	*energysource.PvBase
	meter *carloGavazziModbusMeter
}

type carloGavazziModbusMeter struct {
	modbusUnitId uint8
	lineIndexes  []uint8
	meterCode    string
	meterType    string
	phases       uint8
}

func (c *carloGavazziModbusMeter) initialize(modbusClient *modbus.ModbusClient, modbusMeter *ModbusMeter) error {
	c.modbusUnitId = modbusMeter.ModbusUnitId
	c.lineIndexes = modbusMeter.LineIndexes
	modbusClient.SetUnitId(c.modbusUnitId)
	meterType, err := modbusClient.ReadRegister(0x000B, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	c.meterCode = fmt.Sprintf("%d", meterType)
	switch meterType {
	case 71:
		c.meterType = "EM24-DIN AV"
		c.phases = 3
	case 72:
		c.meterType = "EM24-DIN AV5"
		c.phases = 3
	case 73:
		c.meterType = "EM24-DIN AV6"
		c.phases = 3
	case 100:
		c.meterType = "EM110-DIN AV7 1 x S1"
		c.phases = 1
	case 101:
		c.meterType = "EM111-DIN AV7 1 x S1"
		c.phases = 1
	case 102:
		c.meterType = "EM112-DIN AV1 1 x S1"
		c.phases = 1
	case 103:
		c.meterType = "EM111-DIN AV8 1 x S1"
		c.phases = 1
	case 104:
		c.meterType = "EM112-DIN AV0 1 x S1"
		c.phases = 1
	case 110:
		c.meterType = "EM110-DIN AV8 1 x S1"
		c.phases = 1
	case 114:
		c.meterType = "EM111-DIN AV5 1 X S1 X"
		c.phases = 1
	case 120:
		c.meterType = "ET112-DIN AV0 1 x S1 X"
		c.phases = 1
	case 121:
		c.meterType = "ET112-DIN AV1 1 x S1 X"
		c.phases = 1
	case 331:
		c.meterType = "EM330-DIN AV6 3"
		c.phases = 3
	case 332:
		c.meterType = "EM330-DIN AV5 3"
		c.phases = 3
	case 335:
		c.meterType = "ET330-DIN AV5 3"
		c.phases = 3
	case 336:
		c.meterType = "ET330-DIN AV6 3"
		c.phases = 3
	case 340:
		c.meterType = "EM340-DIN AV2 3 X S1 X"
		c.phases = 3
	case 341:
		c.meterType = "EM340-DIN AV2 3 X S1"
		c.phases = 3
	case 345:
		c.meterType = "ET340-DIN AV2 3 X S1 X"
		c.phases = 3
	case 346:
		c.meterType = "EM341-DIN AV2 3 X OS X"
		c.phases = 3
	case 1744:
		c.meterType = "EM530-DIN AV5 3 X S1 X"
		c.phases = 3
	case 1745:
		c.meterType = "EM530-DIN AV5 3 X S1 PF A"
		c.phases = 3
	case 1746:
		c.meterType = "EM530-DIN AV5 3 X S1 PF B"
		c.phases = 3
	case 1747:
		c.meterType = "EM530-DIN AV5 3 X S1 PF C"
		c.phases = 3
	case 1760:
		c.meterType = "EM540-DIN AV2 3 X S1 X"
		c.phases = 3
	case 1761:
		c.meterType = "EM540-DIN AV2 3 X S1 PF A"
		c.phases = 3
	case 1762:
		c.meterType = "EM540-DIN AV2 3 X S1 PF B"
		c.phases = 3
	case 1763:
		c.meterType = "EM540-DIN AV2 3 X S1 PF C"
		c.phases = 3
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

type CarloGavazziConfig struct {
	ModbusUrl        string
	ModbusGridConfig *ModbusGridConfig
	ModbusPvConfigs  []*ModbusPvConfig
}

func NewCarloGavazziSystem(config *CarloGavazziConfig) (*energysource.System, error) {
	modbusConfig := &modbus.ClientConfiguration{
		URL:     config.ModbusUrl,
		Timeout: time.Millisecond * 500,
		Speed:   9600,
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
		meter := &carloGavazziModbusMeter{}
		err := meter.initialize(modbusClient, config.ModbusGridConfig.ModbusMeter)
		if err != nil {
			return nil, err
		}
		g := energysource.Grid(carloGavazziGrid{
			GridBase: energysource.NewGridBase(config.ModbusGridConfig.GridConfig),
			meter:    meter,
		})
		grid = &g
	}
	var pvs []*energysource.Pv = nil
	if config.ModbusPvConfigs != nil {
		for ix := 0; ix < len(config.ModbusPvConfigs); ix++ {
			meter := &carloGavazziModbusMeter{}
			err := meter.initialize(modbusClient, config.ModbusPvConfigs[ix].ModbusMeter)
			if err != nil {
				return nil, err
			}
			pv := energysource.Pv(carloGavazziPv{
				PvBase: energysource.NewPvBase(config.ModbusPvConfigs[ix].PvConfig),
				meter:  meter,
			})
			pvs = append(pvs, &pv)
		}
	}
	system := energysource.NewSystem(grid, pvs)
	cgSystem := &carloGavazziSystem{}
	go cgSystem.readSystemValues(modbusClient, system)
	return system, nil
}

func (c *carloGavazziSystem) readSystemValues(client *modbus.ModbusClient, system *energysource.System) {
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
				cgGrid, ok := (*system.Grid()).(carloGavazziGrid)
				if ok {
					cgGrid.meter.updateValues(client, cgGrid.EnergyFlowBase)
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					cgPv, ok := (*system.Pvs()[ix]).(carloGavazziPv)
					if ok {
						cgPv.meter.updateValues(client, cgPv.EnergyFlowBase)
					}
				}
			}
		case <-tickerChannel:
			return
		}
	}
}

func (c *carloGavazziModbusMeter) updateValues(modbusClient *modbus.ModbusClient, flow *energysource.EnergyFlowBase) {
	modbusClient.SetUnitId(c.modbusUnitId)
	if 3 == c.phases {
		values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndexes); ix++ {
			offset := c.lineIndexes[ix] * 2
			_ = flow.SetVoltage(c.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset, 10, 0))
		}
		values, _ = modbusClient.ReadRegisters(12, 11, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndexes); ix++ {
			offset := c.lineIndexes[ix] * 2
			_ = flow.SetCurrent(c.lineIndexes[ix], modbusClient.ValueFromResultArray(values, offset, 1000, 0))
			_ = flow.SetPower(c.lineIndexes[ix], modbusClient.ValueFromResultArray(values, 6+offset, 10, 0))
		}
	} else {
		values, _ := modbusClient.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
		_ = flow.SetVoltage(c.lineIndexes[0], modbusClient.ValueFromResultArray(values, 0, 10, 0))
		_ = flow.SetCurrent(c.lineIndexes[0], modbusClient.ValueFromResultArray(values, 2, 1000, 0))
		_ = flow.SetPower(c.lineIndexes[0], modbusClient.ValueFromResultArray(values, 4, 10, 0))
	}
}

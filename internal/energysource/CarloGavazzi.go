package energysource

import (
	"enman/internal/log"
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

type carloGavazziModbusMeter struct {
	modbusUnitId uint8
	lineIndexes  []uint8
	meterCode    string
	meterType    string
	phases       uint8
}

func (c *carloGavazziModbusMeter) initialize(modbusClient *modbus.ModbusClient, modbusMeter *ModbusMeterConfig) error {
	c.modbusUnitId = modbusMeter.ModbusUnitId
	c.lineIndexes = modbusMeter.LineIndexes
	modbusClient.SetUnitId(c.modbusUnitId)
	// Read meter type
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
	log.Infof("Detected a %d phase Carlo Gavazzi %s (identification code %d) with unitId %d at %s.", c.phases, c.meterType, meterType, c.modbusUnitId, modbusClient.URL())
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(em24ApplicationRegister, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", c.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(em24FrontSelectorRegister, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if frontSelector == 3 {
				log.Warning("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
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
	cgSystem := &carloGavazziSystem{}
	return NewModbusSystem(
		&ModbusConfig{
			ModbusUrl: config.ModbusUrl,
			Timeout:   time.Millisecond * 500,
			Speed:     9600,
		},
		config.ModbusGridConfig,
		config.ModbusPvConfigs,
		func() modbusMeter {
			return modbusMeter(&carloGavazziModbusMeter{})
		},
		cgSystem.readSystemValues,
	)
}

func (c *carloGavazziSystem) readSystemValues(client *modbus.ModbusClient, system *energysource.System) {
	pollInterval := uint16(250)
	log.Infof("Start polling Carlo Gavazzi modbus devices every %d milliseconds.", pollInterval)
	ticker := time.NewTicker(time.Millisecond * time.Duration(pollInterval))
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
						cgMeter, ok := (*meter).(*carloGavazziModbusMeter)
						if ok {
							cgMeter.updateValues(client, mGrid.EnergyFlowBase)
						}
					}
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					mPv, ok := (*system.Pvs()[ix]).(modbusPv)
					if ok {
						for _, meter := range mPv.meters {
							cgMeter, ok := (*meter).(*carloGavazziModbusMeter)
							if ok {
								cgMeter.updateValues(client, mPv.EnergyFlowBase)
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

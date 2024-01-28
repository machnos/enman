package meters

import (
	"bytes"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
	"strings"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type carloGavazziMeter struct {
	*energyMeter
	*electricityMeter
	*modbusMeter
	readModbusValues func(*domain.ElectricityState, *domain.ElectricityUsage)
}

func newCarloGavazziMeter(modbusClient *modbus.ModbusClient, meterConfig *config.EnergyMeter) (domain.EnergyMeter, error) {
	enMe := newEnergyMeter("Carlo Gavazzi")
	elMe := newElectricityMeter(meterConfig)
	moMe := newModbusMeter(modbusClient, meterConfig.ModbusUnitId)
	cg := &carloGavazziMeter{
		enMe,
		elMe,
		moMe,
		nil,
	}
	return cg, cg.validMeter()
}

func (c *carloGavazziMeter) UpdateValues(state *domain.ElectricityState, usage *domain.ElectricityUsage, _ *domain.GasUsage, _ *domain.WaterUsage) {
	c.readModbusValues(state, usage)
}

func (c *carloGavazziMeter) Shutdown() {
	log.Infof("Shutting down %s meter with unitId %d at %s.", c.Brand(), c.modbusUnitId, c.modbusClient.URL())
	c.modbusMeter.shutdown()
}

func (c *carloGavazziMeter) validMeter() error {
	meterType, err := c.modbusClient.ReadRegister(c.modbusUnitId, 0x000b, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := c.modbusClient.ReadRegister(c.modbusUnitId, em24ApplicationRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if application != em24ApplicationH {
			log.Infof("Detected a %s EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", c.Brand(), c.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := c.modbusClient.ReadRegister(c.modbusUnitId, em24FrontSelectorRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if frontSelector == 3 {
				return fmt.Errorf("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
					"to manually update the EM24 to 'application H', or set the front selector in an unlocked position " +
					"and reinitialize the system")
			} else {
				err := c.modbusClient.WriteRegister(c.modbusUnitId, em24ApplicationRegister, em24ApplicationH, modbus.BIG_ENDIAN)
				if err != nil {
					return err
				}
			}
		}
	}

	switch meterType {
	case 71:
		c.model = "EM24-DIN AV"
		c.phases = 3
		c.serial = c.readEM24Serial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEm24Values
	case 72:
		c.model = "EM24-DIN AV5"
		c.phases = 3
		c.serial = c.readEM24Serial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEm24Values
	case 73:
		c.model = "EM24-DIN AV6"
		c.phases = 3
		c.serial = c.readEM24Serial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEm24Values
	case 100:
		c.model = "EM110-DIN AV7 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 101:
		c.model = "EM111-DIN AV7 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 102:
		c.model = "EM112-DIN AV1 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 103:
		c.model = "EM111-DIN AV8 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 104:
		c.model = "EM112-DIN AV0 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 110:
		c.model = "EM110-DIN AV8 1 x S1"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 114:
		c.model = "EM111-DIN AV5 1 X S1 X"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 120:
		c.model = "ET112-DIN AV0 1 x S1 X"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 121:
		c.model = "ET112-DIN AV1 1 x S1 X"
		c.phases = 1
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 331:
		c.model = "EM330-DIN AV6 3"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 332:
		c.model = "EM330-DIN AV5 3"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 335:
		c.model = "ET330-DIN AV5 3"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 336:
		c.model = "ET330-DIN AV6 3"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 340:
		c.model = "EM340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 341:
		c.model = "EM340-DIN AV2 3 X S1"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 345:
		c.model = "ET340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 346:
		c.model = "EM341-DIN AV2 3 X OS X"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 1744:
		c.model = "EM530-DIN AV5 3 X S1 X"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1745:
		c.model = "EM530-DIN AV5 3 X S1 PF A"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1746:
		c.model = "EM530-DIN AV5 3 X S1 PF B"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1747:
		c.model = "EM530-DIN AV5 3 X S1 PF C"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1760:
		c.model = "EM540-DIN AV2 3 X S1 X"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1761:
		c.model = "EM540-DIN AV2 3 X S1 PF A"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1762:
		c.model = "EM540-DIN AV2 3 X S1 PF B"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1763:
		c.model = "EM540-DIN AV2 3 X S1 PF C"
		c.phases = 3
		c.serial = c.readGenericSerial(c.modbusUnitId, c.modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	default:
		return fmt.Errorf("detected an unsupported %s electricity meter (%d). Meter will not be queried for values", c.Brand(), meterType)
	}
	log.Infof("Detected a %d phase %s %s (identification code %d, serial %s) with unitId %d at %s.", c.phases, c.brand, c.model, meterType, c.serial, c.modbusUnitId, c.modbusClient.URL())
	return nil
}

func (c *carloGavazziMeter) readGenericSerial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
	values, err := modbusClient.ReadRegisters(modbusUnitId, 0x5000, 7, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	return strings.Trim(b.String(), "0")
}

func (c *carloGavazziMeter) readEM24Serial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
	values, err := modbusClient.ReadRegisters(modbusUnitId, 0x1300, 7, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	for _, value := range values {
		bArray := []byte{byte(value >> 8), byte(value & 0xff)}
		b.Write(bArray)
	}
	return strings.Trim(b.String(), "0")
}

func (c *carloGavazziMeter) readEm24Values(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		if len(c.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x003e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x005c, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		}
		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0046, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, c.lineIndices[ix], 10, 0)))
		}
	}
}

func (c *carloGavazziMeter) readEx100SeriesValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericSinglePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0010, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyConsumed(c.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x0020, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyProvided(c.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
	}
}

func (c *carloGavazziMeter) readEx300SeriesValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		if len(c.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0034, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			if !strings.HasPrefix(c.model, "ET") {
				values, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x004e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
				electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
			}
		}

		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0040, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, c.lineIndices[ix], 10, 0)))
		}
		if strings.HasPrefix(c.model, "ET") {
			values, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0060, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			for ix := 0; ix < len(c.lineIndices); ix++ {
				electricityUsage.SetEnergyProvided(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(values, c.lineIndices[ix], 10, 0)))
			}
		}
	}
}

func (c *carloGavazziMeter) readEM530andEM540Values(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		if len(c.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0034, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x004e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		}
		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0040, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, c.lineIndices[ix], 10, 0)))
		}
	}
}

func (c *carloGavazziMeter) readGenericSinglePhaseState(electricityState *domain.ElectricityState) {
	if !c.HasStateAttribute() || electricityState == nil {
		return
	}
	modbusClient := c.modbusClient
	uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0000, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)

	electricityState.SetVoltage(c.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0))
	// First set the power, because on some meters we need to flip the current sign based on power
	electricityState.SetPower(c.lineIndices[0], modbusClient.ValueFromInt32sResultArray(uint32s, 2, 10, 0))

	current := modbusClient.ValueFromInt32sResultArray(uint32s, 1, 1000, 0)
	if electricityState.Power(c.lineIndices[0]) < 0 && current > 0 {
		current = current * -1
	}
	electricityState.SetCurrent(c.lineIndices[0], current)
}

func (c *carloGavazziMeter) readGenericThreePhaseState(electricityState *domain.ElectricityState) {
	if !c.HasStateAttribute() || electricityState == nil {
		return
	}
	modbusClient := c.modbusClient
	uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0000, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(c.lineIndices); ix++ {
		electricityState.SetVoltage(c.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, c.lineIndices[ix], 10, 0))
	}
	uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x0012, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(c.lineIndices); ix++ {
		// First set the power, because on some meters we need to flip the current sign based on power
		electricityState.SetPower(c.lineIndices[ix], modbusClient.ValueFromInt32sResultArray(uint32s, c.lineIndices[ix], 10, 0))
	}
	uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x000c, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(c.lineIndices); ix++ {
		current := modbusClient.ValueFromInt32sResultArray(uint32s, c.lineIndices[ix], 1000, 0)
		if electricityState.Power(c.lineIndices[ix]) < 0 && current > 0 {
			current = current * -1
		}
		electricityState.SetCurrent(c.lineIndices[ix], current)
	}
}

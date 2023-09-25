package meters_old

import (
	"bytes"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"strings"
)

const (
	em24FrontSelectorRegister = 0x0304
	em24ApplicationRegister   = 0x1101
	em24ApplicationH          = uint16(7)
)

type carloGavazzi struct {
	*genericEnergyMeter
	*genericElectricityMeter
	*modbusEnergyMeter
}

func newCarloGavazzi(genEnMe *genericEnergyMeter, genElMe *genericElectricityMeter, moElMe *modbusEnergyMeter) *carloGavazzi {
	return &carloGavazzi{
		genEnMe, genElMe, moElMe,
	}
}

func (c *carloGavazzi) probe(modbusClient *modbus.ModbusClient, meterType uint16) bool {
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(c.modbusUnitId, em24ApplicationRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err != nil {
			log.Error(err.Error())
			return false
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", c.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(c.modbusUnitId, em24FrontSelectorRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
			if err != nil {
				log.Error(err.Error())
				return false
			}
			if frontSelector == 3 {
				log.Warning("EM24 front selector is locked. Cannot update application to 'H'. Please use the joystick " +
					"to manually update the EM24 to 'application H', or set the front selector in an unlocked position " +
					"and reinitialize the system.")
				return false
			} else {
				err := modbusClient.WriteRegister(c.modbusUnitId, em24ApplicationRegister, em24ApplicationH)
				if err != nil {
					log.Error(err.Error())
					return false
				}
			}
		}
	}

	switch meterType {
	case 71:
		c.meterType = "EM24-DIN AV"
		c.phases = 3
		c.meterSerial = c.readEM24Serial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEm24Values
	case 72:
		c.meterType = "EM24-DIN AV5"
		c.phases = 3
		c.meterSerial = c.readEM24Serial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEm24Values
	case 73:
		c.meterType = "EM24-DIN AV6"
		c.phases = 3
		c.meterSerial = c.readEM24Serial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEm24Values
	case 100:
		c.meterType = "EM110-DIN AV7 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 101:
		c.meterType = "EM111-DIN AV7 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 102:
		c.meterType = "EM112-DIN AV1 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 103:
		c.meterType = "EM111-DIN AV8 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 104:
		c.meterType = "EM112-DIN AV0 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 110:
		c.meterType = "EM110-DIN AV8 1 x S1"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 114:
		c.meterType = "EM111-DIN AV5 1 X S1 X"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 120:
		c.meterType = "ET112-DIN AV0 1 x S1 X"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 121:
		c.meterType = "ET112-DIN AV1 1 x S1 X"
		c.phases = 1
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx100SeriesValues
	case 331:
		c.meterType = "EM330-DIN AV6 3"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 332:
		c.meterType = "EM330-DIN AV5 3"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 335:
		c.meterType = "ET330-DIN AV5 3"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 336:
		c.meterType = "ET330-DIN AV6 3"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 340:
		c.meterType = "EM340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 341:
		c.meterType = "EM340-DIN AV2 3 X S1"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 345:
		c.meterType = "ET340-DIN AV2 3 X S1 X"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 346:
		c.meterType = "EM341-DIN AV2 3 X OS X"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEx300SeriesValues
	case 1744:
		c.meterType = "EM530-DIN AV5 3 X S1 X"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1745:
		c.meterType = "EM530-DIN AV5 3 X S1 PF A"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1746:
		c.meterType = "EM530-DIN AV5 3 X S1 PF B"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1747:
		c.meterType = "EM530-DIN AV5 3 X S1 PF C"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1760:
		c.meterType = "EM540-DIN AV2 3 X S1 X"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1761:
		c.meterType = "EM540-DIN AV2 3 X S1 PF A"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1762:
		c.meterType = "EM540-DIN AV2 3 X S1 PF B"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	case 1763:
		c.meterType = "EM540-DIN AV2 3 X S1 PF C"
		c.phases = 3
		c.meterSerial = c.readGenericSerial(c.modbusUnitId, modbusClient)
		c.readModbusValues = c.readEM530andEM540Values
	default:
		log.Warningf("Detected an unsupported Carlo Gavazzi electricity meter (%d). Meter will not be queried for values.", meterType)
		return false
	}
	c.meterBrand = "Carlo Gavazzi"
	c.modbusClient = modbusClient
	log.Infof("Detected a %d phase %s %s (identification code %d, serial %s) with unitId %d at %s.", c.phases, c.meterBrand, c.meterType, meterType, c.meterSerial, c.modbusUnitId, modbusClient.URL())
	return true
}

func (c *carloGavazzi) readGenericSerial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
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

func (c *carloGavazzi) readEM24Serial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
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

func (c *carloGavazzi) readEm24Values(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
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

func (c *carloGavazzi) readEx100SeriesValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericSinglePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0010, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyConsumed(c.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		uint32s, _ = modbusClient.ReadUint32s(c.modbusUnitId, 0x0020, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyProvided(c.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
	}
}

func (c *carloGavazzi) readEx300SeriesValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(electricityState)

	if c.HasUsageAttribute() && c.shouldUpdateUsage() && electricityUsage != nil {
		modbusClient := c.modbusClient
		if len(c.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0034, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			if !strings.HasPrefix(c.meterType, "ET") {
				values, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x004e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
				electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
			}
		}

		uint32s, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0040, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(c.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, c.lineIndices[ix], 10, 0)))
		}
		if strings.HasPrefix(c.meterType, "ET") {
			values, _ := modbusClient.ReadUint32s(c.modbusUnitId, 0x0060, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			for ix := 0; ix < len(c.lineIndices); ix++ {
				electricityUsage.SetEnergyProvided(c.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(values, c.lineIndices[ix], 10, 0)))
			}
		}
	}
}

func (c *carloGavazzi) readEM530andEM540Values(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
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

func (c *carloGavazzi) readGenericSinglePhaseState(electricityState *domain.ElectricityState) {
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

func (c *carloGavazzi) readGenericThreePhaseState(electricityState *domain.ElectricityState) {
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

func (c *carloGavazzi) enrichMeterValues(electricityMeterValues *domain.ElectricityMeterValues, _ *domain.GasMeterValues, _ *domain.WaterMeterValues) {
	electricityMeterValues.
		SetRole(c.role).
		SetMeterPhases(c.phases).
		SetMeterSerial(c.meterSerial).
		SetMeterType(c.meterType).
		SetMeterBrand(c.meterBrand)
}

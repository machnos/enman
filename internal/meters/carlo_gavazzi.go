package meters

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

type carlo_gavazzi struct {
}

func (c *carlo_gavazzi) probe(meter *electricityModbusMeter, modbusClient *modbus.ModbusClient, meterType uint16) bool {
	if meterType >= 71 && meterType <= 73 {
		// type EM24 detected. Check if application is set to 'H'.
		application, err := modbusClient.ReadRegister(meter.modbusUnitId, em24ApplicationRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err != nil {
			log.Error(err.Error())
			return false
		}
		if application != em24ApplicationH {
			log.Infof("Detected a Carlo Gavazzi EM24 with unitId %d that is not configured as 'Application H'. "+
				"Trying to set application mode to 'Application H'.", meter.modbusUnitId)
			// Application not set to 'H'. Check if we can update the value.
			frontSelector, err := modbusClient.ReadRegister(meter.modbusUnitId, em24FrontSelectorRegister, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
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
				err := modbusClient.WriteRegister(meter.modbusUnitId, em24ApplicationRegister, em24ApplicationH)
				if err != nil {
					log.Error(err.Error())
					return false
				}
			}
		}
	}

	switch meterType {
	case 71:
		meter.meterType = "EM24-DIN AV"
		meter.phases = 3
		meter.meterSerial = c.readEM24Serial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEm24Values
	case 72:
		meter.meterType = "EM24-DIN AV5"
		meter.phases = 3
		meter.meterSerial = c.readEM24Serial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEm24Values
	case 73:
		meter.meterType = "EM24-DIN AV6"
		meter.phases = 3
		meter.meterSerial = c.readEM24Serial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEm24Values
	case 100:
		meter.meterType = "EM110-DIN AV7 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 101:
		meter.meterType = "EM111-DIN AV7 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 102:
		meter.meterType = "EM112-DIN AV1 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 103:
		meter.meterType = "EM111-DIN AV8 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 104:
		meter.meterType = "EM112-DIN AV0 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 110:
		meter.meterType = "EM110-DIN AV8 1 x S1"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 114:
		meter.meterType = "EM111-DIN AV5 1 X S1 X"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 120:
		meter.meterType = "ET112-DIN AV0 1 x S1 X"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 121:
		meter.meterType = "ET112-DIN AV1 1 x S1 X"
		meter.phases = 1
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx100SeriesValues
	case 331:
		meter.meterType = "EM330-DIN AV6 3"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 332:
		meter.meterType = "EM330-DIN AV5 3"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 335:
		meter.meterType = "ET330-DIN AV5 3"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 336:
		meter.meterType = "ET330-DIN AV6 3"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 340:
		meter.meterType = "EM340-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 341:
		meter.meterType = "EM340-DIN AV2 3 X S1"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 345:
		meter.meterType = "ET340-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 346:
		meter.meterType = "EM341-DIN AV2 3 X OS X"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEx300SeriesValues
	case 1744:
		meter.meterType = "EM530-DIN AV5 3 X S1 X"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1745:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF A"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1746:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF B"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1747:
		meter.meterType = "EM530-DIN AV5 3 X S1 PF C"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1760:
		meter.meterType = "EM540-DIN AV2 3 X S1 X"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1761:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF A"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1762:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF B"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	case 1763:
		meter.meterType = "EM540-DIN AV2 3 X S1 PF C"
		meter.phases = 3
		meter.meterSerial = c.readGenericSerial(meter.modbusUnitId, modbusClient)
		meter.readValues = c.readEM530andEM540Values
	default:
		log.Warningf("Detected an unsupported Carlo Gavazzi electricity meter (%d). Meter will not be queried for values.", meterType)
		return false
	}
	meter.meterBrand = "Carlo Gavazzi"
	meter.modbusClient = modbusClient
	log.Infof("Detected a %d phase %s %s (identification code %d, serial %s) with unitId %d at %s.", meter.phases, meter.meterBrand, meter.meterType, meterType, meter.meterSerial, meter.modbusUnitId, modbusClient.URL())
	return true
}

func (c *carlo_gavazzi) readGenericSerial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
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

func (c *carlo_gavazzi) readEM24Serial(modbusUnitId uint8, modbusClient *modbus.ModbusClient) string {
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

func (c *carlo_gavazzi) readEm24Values(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(meter, electricityState)

	if meter.HasUsageAttribute() && electricityUsage != nil {
		modbusClient := meter.modbusClient
		if len(meter.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x003e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x005c, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		}
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0046, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 10, 0)))
		}
	}
}

func (c *carlo_gavazzi) readEx100SeriesValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericSinglePhaseState(meter, electricityState)

	if meter.HasUsageAttribute() && electricityUsage != nil {
		modbusClient := meter.modbusClient
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0010, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyConsumed(meter.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0020, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		electricityUsage.SetEnergyProvided(meter.lineIndices[0], float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
	}
}

func (c *carlo_gavazzi) readEx300SeriesValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(meter, electricityState)

	if meter.HasUsageAttribute() && electricityUsage != nil {
		modbusClient := meter.modbusClient
		if len(meter.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0034, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			if !strings.HasPrefix(meter.meterType, "ET") {
				values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x004e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
				electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(values, 0, 10, 0)))
			}
		}

		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0040, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 10, 0)))
		}
		if strings.HasPrefix(meter.meterType, "ET") {
			values, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0060, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			for ix := 0; ix < len(meter.lineIndices); ix++ {
				electricityUsage.SetEnergyProvided(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(values, meter.lineIndices[ix], 10, 0)))
			}
		}
	}
}

func (c *carlo_gavazzi) readEM530andEM540Values(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	c.readGenericThreePhaseState(meter, electricityState)

	if meter.HasUsageAttribute() && electricityUsage != nil {
		modbusClient := meter.modbusClient
		if len(meter.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0034, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyConsumed(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
			uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x004e, 1, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
			electricityUsage.SetTotalEnergyProvided(float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0)))
		}
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0040, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 10, 0)))
		}
	}
}

func (c *carlo_gavazzi) readGenericSinglePhaseState(meter *electricityModbusMeter, electricityState *domain.ElectricityState) {
	if !meter.HasStateAttribute() || electricityState == nil {
		return
	}
	modbusClient := meter.modbusClient
	uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0000, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)

	electricityState.SetVoltage(meter.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0))
	// First set the power, because on some meters we need to flip the current sign based on power
	electricityState.SetPower(meter.lineIndices[0], modbusClient.ValueFromInt32sResultArray(uint32s, 2, 10, 0))

	current := modbusClient.ValueFromInt32sResultArray(uint32s, 1, 1000, 0)
	if electricityState.Power(meter.lineIndices[0]) < 0 && current > 0 {
		current = current * -1
	}
	electricityState.SetCurrent(meter.lineIndices[0], current)
}

func (c *carlo_gavazzi) readGenericThreePhaseState(meter *electricityModbusMeter, electricityState *domain.ElectricityState) {
	if !meter.HasStateAttribute() || electricityState == nil {
		return
	}
	modbusClient := meter.modbusClient
	uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x0000, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndices); ix++ {
		electricityState.SetVoltage(meter.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 10, 0))
	}
	uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x0012, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndices); ix++ {
		// First set the power, because on some meters we need to flip the current sign based on power
		electricityState.SetPower(meter.lineIndices[ix], modbusClient.ValueFromInt32sResultArray(uint32s, meter.lineIndices[ix], 10, 0))
	}
	uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x000c, 3, modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, modbus.INPUT_REGISTER)
	for ix := 0; ix < len(meter.lineIndices); ix++ {
		current := modbusClient.ValueFromInt32sResultArray(uint32s, meter.lineIndices[ix], 1000, 0)
		if electricityState.Power(meter.lineIndices[ix]) < 0 && current > 0 {
			current = current * -1
		}
		electricityState.SetCurrent(meter.lineIndices[ix], current)
	}
}

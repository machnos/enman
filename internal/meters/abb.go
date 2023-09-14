package meters

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
)

type abb struct {
}

func (a *abb) probe(meter *electricityModbusMeter, modbusClient *modbus.ModbusClient, meterType uint32) bool {
	switch meterType {
	case 0x42323120:
		meter.meterType = "B21"
		meter.phases = 1
		meter.readValues = a.readSinglePhaseValues
	case 0x42323320:
		meter.meterType = "B23"
		meter.phases = 3
		meter.readValues = a.readThreePhaseValues
	case 0x42323420:
		meter.meterType = "B24"
		meter.phases = 3
		meter.readValues = a.readThreePhaseValues
	default:
		log.Warningf("Detected an unsupported ABB electricity meter (%d). Meter will not be queried for values.", meterType)
		return false
	}
	meter.modbusClient = modbusClient
	meter.meterBrand = "ABB"
	log.Infof("Detected a %d phase %s %s (identification code %d) with unitId %d at %s.", meter.phases, meter.meterBrand, meter.meterType, meterType, meter.modbusUnitId, meter.modbusClient.URL())
	return true
}

func (a *abb) readSinglePhaseValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := meter.modbusClient
	if meter.HasStateAttribute() && electricityState != nil {
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b00, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetVoltage(meter.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0))
		uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b0c, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetCurrent(meter.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 100, 0))
		uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b14, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetPower(meter.lineIndices[0], modbusClient.ValueFromInt32sResultArray(uint32s, 0, 100, 0))
	}
	if meter.HasUsageAttribute() && electricityUsage != nil {
		uint64s, _ := modbusClient.ReadUint64s(meter.modbusUnitId, 0x5000, 2, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityUsage.SetEnergyConsumed(meter.lineIndices[0], modbusClient.ValueFromUint64sResultArray(uint64s, 0, 100, 0))
		electricityUsage.SetEnergyProvided(meter.lineIndices[0], modbusClient.ValueFromUint64sResultArray(uint64s, 1, 100, 0))
	}
}

func (a *abb) readThreePhaseValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := meter.modbusClient
	if meter.HasStateAttribute() && electricityState != nil {
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b00, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityState.SetVoltage(meter.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 10, 0))
		}
		uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b0c, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityState.SetCurrent(meter.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 100, 0))
		}
		uint32s, _ = modbusClient.ReadUint32s(meter.modbusUnitId, 0x5b16, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityState.SetPower(meter.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 100, 0))
		}
	}
	if meter.HasUsageAttribute() && electricityUsage != nil {
		uint64s, _ := modbusClient.ReadUint64s(meter.modbusUnitId, 0x5000, 2, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		if len(meter.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			electricityUsage.SetTotalEnergyConsumed(modbusClient.ValueFromUint64sResultArray(uint64s, 0, 100, 0))
			electricityUsage.SetTotalEnergyProvided(modbusClient.ValueFromUint64sResultArray(uint64s, 1, 100, 0))
		}
		uint64s, _ = modbusClient.ReadUint64s(meter.modbusUnitId, 0x5460, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], modbusClient.ValueFromUint64sResultArray(uint64s, meter.lineIndices[ix], 100, 0))
		}
		uint64s, _ = modbusClient.ReadUint64s(meter.modbusUnitId, 0x546c, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyProvided(meter.lineIndices[ix], modbusClient.ValueFromUint64sResultArray(uint64s, meter.lineIndices[ix], 100, 0))
		}
	}
}

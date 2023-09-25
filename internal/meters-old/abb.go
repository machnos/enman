package meters_old

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
)

type abb struct {
	*genericEnergyMeter
	*genericElectricityMeter
	*modbusEnergyMeter
}

func newAbb(genEnMe *genericEnergyMeter, genElMe *genericElectricityMeter, moElMe *modbusEnergyMeter) *abb {
	return &abb{
		genEnMe,
		genElMe,
		moElMe,
	}
}

func (a *abb) probe(modbusClient *modbus.ModbusClient, meterType uint32) bool {
	switch meterType {
	case 0x42323120:
		a.meterType = "B21"
		a.phases = 1
		a.readModbusValues = a.readSinglePhaseValues
	case 0x42323320:
		a.meterType = "B23"
		a.phases = 3
		a.readModbusValues = a.readThreePhaseValues
	case 0x42323420:
		a.meterType = "B24"
		a.phases = 3
		a.readModbusValues = a.readThreePhaseValues
	default:
		log.Warningf("Detected an unsupported ABB electricity meter (%d). Meter will not be queried for values.", meterType)
		return false
	}
	a.modbusClient = modbusClient
	log.Infof("Detected a %d phase %s %s (identification code %d) with unitId %d at %s.", a.phases, a.meterBrand, a.meterType, meterType, a.modbusUnitId, a.modbusClient.URL())
	return true
}

func (a *abb) readSinglePhaseValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := a.modbusClient
	if a.HasStateAttribute() && electricityState != nil {
		uint32s, _ := modbusClient.ReadUint32s(a.modbusUnitId, 0x5b00, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetVoltage(a.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 10, 0))
		uint32s, _ = modbusClient.ReadUint32s(a.modbusUnitId, 0x5b0c, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetCurrent(a.lineIndices[0], modbusClient.ValueFromUint32sResultArray(uint32s, 0, 100, 0))
		uint32s, _ = modbusClient.ReadUint32s(a.modbusUnitId, 0x5b14, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityState.SetPower(a.lineIndices[0], modbusClient.ValueFromInt32sResultArray(uint32s, 0, 100, 0))
	}
	if a.HasUsageAttribute() && a.shouldUpdateUsage() && electricityUsage != nil {
		uint64s, _ := modbusClient.ReadUint64s(a.modbusUnitId, 0x5000, 2, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		electricityUsage.SetEnergyConsumed(a.lineIndices[0], modbusClient.ValueFromUint64sResultArray(uint64s, 0, 100, 0))
		electricityUsage.SetEnergyProvided(a.lineIndices[0], modbusClient.ValueFromUint64sResultArray(uint64s, 1, 100, 0))
	}
}

func (a *abb) readThreePhaseValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := a.modbusClient
	if a.HasStateAttribute() && electricityState != nil {
		uint32s, _ := modbusClient.ReadUint32s(a.modbusUnitId, 0x5b00, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(a.lineIndices); ix++ {
			electricityState.SetVoltage(a.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, a.lineIndices[ix], 10, 0))
		}
		uint32s, _ = modbusClient.ReadUint32s(a.modbusUnitId, 0x5b0c, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(a.lineIndices); ix++ {
			electricityState.SetCurrent(a.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, a.lineIndices[ix], 100, 0))
		}
		uint32s, _ = modbusClient.ReadUint32s(a.modbusUnitId, 0x5b16, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(a.lineIndices); ix++ {
			electricityState.SetPower(a.lineIndices[ix], modbusClient.ValueFromUint32sResultArray(uint32s, a.lineIndices[ix], 100, 0))
		}
	}
	if a.HasUsageAttribute() && a.shouldUpdateUsage() && electricityUsage != nil {
		uint64s, _ := modbusClient.ReadUint64s(a.modbusUnitId, 0x5000, 2, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		if len(a.lineIndices) == 3 {
			// Only set totals when all line indices are configured
			electricityUsage.SetTotalEnergyConsumed(modbusClient.ValueFromUint64sResultArray(uint64s, 0, 100, 0))
			electricityUsage.SetTotalEnergyProvided(modbusClient.ValueFromUint64sResultArray(uint64s, 1, 100, 0))
		}
		uint64s, _ = modbusClient.ReadUint64s(a.modbusUnitId, 0x5460, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(a.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(a.lineIndices[ix], modbusClient.ValueFromUint64sResultArray(uint64s, a.lineIndices[ix], 100, 0))
		}
		uint64s, _ = modbusClient.ReadUint64s(a.modbusUnitId, 0x546c, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
		for ix := 0; ix < len(a.lineIndices); ix++ {
			electricityUsage.SetEnergyProvided(a.lineIndices[ix], modbusClient.ValueFromUint64sResultArray(uint64s, a.lineIndices[ix], 100, 0))
		}
	}
}

func (a *abb) enrichMeterValues(electricityMeterValues *domain.ElectricityMeterValues, _ *domain.GasMeterValues, _ *domain.WaterMeterValues) {
	electricityMeterValues.
		SetRole(a.role).
		SetMeterPhases(a.phases).
		SetMeterSerial(a.meterSerial).
		SetMeterType(a.meterType).
		SetMeterBrand(a.meterBrand)
}

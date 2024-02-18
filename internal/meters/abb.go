package meters

import (
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
)

type abbMeter struct {
	*energyMeter
	*electricityMeter
	*modbusMeter
	readModbusValues func(*domain.ElectricityState, *domain.ElectricityUsage)
}

func newAbbMeter(modbusClient *modbus.ModbusClient, meterConfig *config.EnergyMeter) (domain.EnergyMeter, error) {
	enMe := newEnergyMeter("ABB")
	elMe := newElectricityMeter(meterConfig)
	moMe := newModbusMeter(modbusClient, meterConfig.ModbusUnitId)
	abb := &abbMeter{
		enMe,
		elMe,
		moMe,
		nil,
	}
	return abb, abb.validMeter()
}

func (a *abbMeter) UpdateValues(state *domain.ElectricityState, usage *domain.ElectricityUsage, _ *domain.GasUsage, _ *domain.WaterUsage, _ *domain.BatteryState) {
	a.readModbusValues(state, usage)
}
func (a *abbMeter) Shutdown() {
	log.Infof("Shutting down %s meter with unitId %d at %s.", a.Brand(), a.modbusUnitId, a.modbusClient.URL())
	a.modbusMeter.shutdown()
}

func (a *abbMeter) validMeter() error {
	meterType, err := a.modbusClient.ReadUint32(a.modbusUnitId, 0x8960, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.HOLDING_REGISTER)
	if err != nil {
		return err
	}
	switch meterType {
	case 0x42323120:
		a.model = "B21"
		a.phases = 1
		a.readModbusValues = a.readSinglePhaseValues
	case 0x42323320:
		a.model = "B23"
		a.phases = 3
		a.readModbusValues = a.readThreePhaseValues
	case 0x42323420:
		a.model = "B24"
		a.phases = 3
		a.readModbusValues = a.readThreePhaseValues
	default:
		return fmt.Errorf("detected an unsupported %s electricity meter (%d). Meter will not be queried for values", a.Brand(), meterType)
	}
	log.Infof("detected a %d phase %s %s (identification code %d) with unitId %d at %s.", a.phases, a.brand, a.model, meterType, a.modbusUnitId, a.modbusClient.URL())
	a.setDefaultLineIndices(fmt.Sprintf("%d phase %s %s with unitId %d at %s", a.phases, a.brand, a.model, a.modbusUnitId, a.modbusClient.URL()))
	return nil
}

func (a *abbMeter) readSinglePhaseValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
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

func (a *abbMeter) readThreePhaseValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
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

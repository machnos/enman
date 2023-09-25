package meters_old

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
)

type victron struct {
	*genericEnergyMeter
	*genericElectricityMeter
	*modbusEnergyMeter
}

func newVictron(genEnMe *genericEnergyMeter, genElMe *genericElectricityMeter, moElMe *modbusEnergyMeter) *victron {
	return &victron{
		genEnMe, genElMe, moElMe,
	}
}

func (v *victron) probe(modbusClient *modbus.ModbusClient, meterRole domain.EnergySourceRole) bool {
	switch meterRole {
	case domain.RoleGrid:
		v.meterType = "Victron Grid"
		v.phases = v.probePhases(v.modbusUnitId, modbusClient, []uint16{2616, 2618, 2620})
		v.meterSerial = v.probeSerial(v.modbusUnitId, modbusClient, 2609)
		v.readModbusValues = v.readGridValues
	case domain.RolePv:
		v.meterType = "Victron PV"
		v.phases = v.probePhases(v.modbusUnitId, modbusClient, []uint16{1027, 1031, 1035})
		// address 1309 gives a weird error in v3.01 of victron. A bug should be raised, because victron thinks it needs to be a battery instead of PV.
		//v.meterSerial = v.probeSerial(v.modbusUnitId, modbusClient, 1309)
		v.readModbusValues = v.readPvValues
	default:
		log.Warningf("Detected an unsupported Victron meter (%v). Meter will not be queried for values.", meterRole)
		return false
	}
	v.modbusClient = modbusClient
	v.meterBrand = "Victron"
	log.Infof("Detected a %d phase %s with unitId %d at %s.", v.phases, v.meterType, v.modbusUnitId, v.modbusClient.URL())
	return true
}

func (v *victron) probePhases(modbusUnitId uint8, modbusClient *modbus.ModbusClient, addresses []uint16) uint8 {
	phases := uint8(0)
	for _, address := range addresses {
		values, _ := modbusClient.ReadRegisters(modbusUnitId, address, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		voltage := modbusClient.ValueFromUint16sResultArray(values, 0, 10, 0)
		if voltage > 0 {
			phases++
		}
	}
	return phases
}

func (v *victron) probeSerial(modbusUnitId uint8, modbusClient *modbus.ModbusClient, address uint16) string {
	bytes, err := modbusClient.ReadBytes(modbusUnitId, address, 14, modbus.INPUT_REGISTER)
	if err != nil {
		log.Warningf("Unable to read Victron serial: %s", err.Error())
		return ""
	}
	return string(bytes)
}

func (v *victron) readGridValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := v.modbusClient
	if v.HasStateAttribute() && electricityState != nil {
		uint16s, _ := modbusClient.ReadRegisters(v.modbusUnitId, 2600, 3, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(v.lineIndices); ix++ {
			electricityState.SetPower(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, v.lineIndices[ix], 0, 0))
		}
		uint16s, _ = modbusClient.ReadRegisters(v.modbusUnitId, 2616, 6, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(v.lineIndices); ix++ {
			offset := v.lineIndices[ix] * 2
			electricityState.SetVoltage(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
			electricityState.SetCurrent(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
		}
	}

	if v.HasUsageAttribute() && v.shouldUpdateUsage() && electricityUsage != nil {
		uint32s, _ := modbusClient.ReadUint32s(v.modbusUnitId, 2622, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(v.lineIndices); ix++ {
			offset := v.lineIndices[ix]
			electricityUsage.SetEnergyConsumed(v.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, offset, 100, 0)))
		}
		uint32s, err := modbusClient.ReadUint32s(v.modbusUnitId, 2636, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		if uint32s != nil && err == nil {
			// Provided energy per phase is far from correct, so we split the total energy (which seems to be correct) equally over the given phases.
			provided := float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 100, 0))
			providedPerPhase := provided / float64(len(v.lineIndices))
			for ix := 0; ix < len(v.lineIndices); ix++ {
				electricityUsage.SetEnergyProvided(v.lineIndices[ix], providedPerPhase)
			}
		}
	}
}

func (v *victron) readPvValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := v.modbusClient
	if v.HasStateAttribute() && electricityState != nil {
		uint16s, _ := modbusClient.ReadRegisters(v.modbusUnitId, 1027, 11, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(v.lineIndices); ix++ {
			offset := v.lineIndices[ix] * 4
			electricityState.SetVoltage(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
			electricityState.SetCurrent(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
			electricityState.SetPower(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+2, 0, 0))
		}
	}
	if v.HasUsageAttribute() && v.shouldUpdateUsage() && electricityUsage != nil {
		uint32s, err := modbusClient.ReadUint32s(v.modbusUnitId, 1046, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		if err != nil || uint32s == nil || len(uint32s) < 3 {
			return
		}
		for ix := 0; ix < len(v.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(v.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, v.lineIndices[ix], 100, 0)))
		}
	}
}

func (v *victron) enrichMeterValues(electricityMeterValues *domain.ElectricityMeterValues, _ *domain.GasMeterValues, _ *domain.WaterMeterValues) {
	electricityMeterValues.
		SetRole(v.role).
		SetMeterPhases(v.phases).
		SetMeterSerial(v.meterSerial).
		SetMeterType(v.meterType).
		SetMeterBrand(v.meterBrand)
}

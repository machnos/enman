package meters

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
)

type victron struct {
}

func (v *victron) probe(meter *electricityModbusMeter, modbusClient *modbus.ModbusClient, meterType domain.ElectricitySourceRole) bool {
	switch meterType {
	case domain.RoleGrid:
		meter.meterType = "Victron Grid"
		meter.phases = v.probePhases(meter.modbusUnitId, modbusClient, []uint16{2616, 2618, 2620})
		meter.meterSerial = v.probeSerial(meter.modbusUnitId, modbusClient, 2609)
		meter.readValues = v.readGridValues
	case domain.RolePv:
		meter.meterType = "Victron PV"
		meter.phases = v.probePhases(meter.modbusUnitId, modbusClient, []uint16{1027, 1031, 1035})
		// address 1309 gives a weird error in v3.01 of victron. A bug should be raised, because victron thinks it needs to be a battery instead of PV.
		//meter.meterSerial = v.probeSerial(meter.modbusUnitId, modbusClient, 1309)
		meter.readValues = v.readPvValues
	default:
		log.Warningf("Detected an unsupported Victron meter (%v). Meter will not be queried for values.", meterType)
		return false
	}
	meter.modbusClient = modbusClient
	meter.meterBrand = "Victron"
	log.Infof("Detected a %d phase %s with unitId %d at %s.", meter.phases, meter.meterType, meter.modbusUnitId, meter.modbusClient.URL())
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

func (v *victron) readGridValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := meter.modbusClient
	if meter.HasStateAttribute() && electricityState != nil {
		uint16s, _ := modbusClient.ReadRegisters(meter.modbusUnitId, 2600, 3, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityState.SetPower(meter.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, meter.lineIndices[ix], 0, 0))
		}
		uint16s, _ = modbusClient.ReadRegisters(meter.modbusUnitId, 2616, 6, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			offset := meter.lineIndices[ix] * 2
			electricityState.SetVoltage(meter.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
			electricityState.SetCurrent(meter.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
		}
	}

	if meter.HasUsageAttribute() && electricityUsage != nil {
		uint32s, _ := modbusClient.ReadUint32s(meter.modbusUnitId, 2622, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			offset := meter.lineIndices[ix]
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, offset, 100, 0)))
		}
		uint32s, err := modbusClient.ReadUint32s(meter.modbusUnitId, 2636, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		if uint32s != nil && err == nil {
			// Provided energy per phase is far from correct, so we split the total energy (which seems to be correct) equally over the given phases.
			provided := float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 100, 0))
			providedPerPhase := provided / float64(len(meter.lineIndices))
			for ix := 0; ix < len(meter.lineIndices); ix++ {
				electricityUsage.SetEnergyProvided(meter.lineIndices[ix], providedPerPhase)
			}
		}
	}
}

func (v *victron) readPvValues(meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
	modbusClient := meter.modbusClient
	if meter.HasStateAttribute() && electricityState != nil {
		uint16s, _ := modbusClient.ReadRegisters(meter.modbusUnitId, 1027, 11, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			offset := meter.lineIndices[ix] * 4
			electricityState.SetVoltage(meter.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
			electricityState.SetCurrent(meter.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
			electricityState.SetPower(meter.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+2, 0, 0))
		}
	}
	if meter.HasUsageAttribute() && electricityUsage != nil {
		uint32s, err := modbusClient.ReadUint32s(meter.modbusUnitId, 1046, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		if err != nil || uint32s == nil || len(uint32s) < 3 {
			return
		}
		for ix := 0; ix < len(meter.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(meter.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, meter.lineIndices[ix], 100, 0)))
		}
	}
}

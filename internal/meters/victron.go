package meters

import (
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
)

type victronMeter struct {
	*energyMeter
	*electricityMeter
	*modbusMeter
	role             domain.EnergySourceRole
	readModbusValues func(*domain.ElectricityState, *domain.ElectricityUsage, *domain.BatteryState) error
}

func newVictronMeter(role domain.EnergySourceRole, modbusClient *modbus.ModbusClient, meterConfig *config.EnergyMeter) (domain.EnergyMeter, error) {
	enMe := newEnergyMeter("Victron")
	elMe := newElectricityMeter(meterConfig)
	moMe := newModbusMeter(modbusClient, meterConfig.ModbusUnitId)
	vm := &victronMeter{
		enMe,
		elMe,
		moMe,
		role,
		nil,
	}
	// TODO if this is a valid meter the targetConsumption from the Grid should be synced with the percentageFromGrid on AcLoad instances.
	return vm, vm.validMeter()
}

func (v *victronMeter) UpdateValues(state *domain.ElectricityState, usage *domain.ElectricityUsage, _ *domain.GasUsage, _ *domain.WaterUsage, batteryState *domain.BatteryState) error {
	return v.readModbusValues(state, usage, batteryState)
}

func (v *victronMeter) Shutdown() {
	log.Infof("Shutting down Victron meter with unitId %d at %s.", v.modbusUnitId, v.modbusClient.URL())
	v.modbusMeter.shutdown()
}

func (v *victronMeter) validMeter() error {
	switch v.role {
	case domain.RoleGrid:
		v.model = "Victron Grid"
		v.phases = v.probePhases(v.modbusUnitId, v.modbusClient, []uint16{2616, 2618, 2620})
		if v.phases == 0 {
			return fmt.Errorf("detected an unsupported %s meter (%v). Meter will not be queried for values", v.Brand(), v.role)
		}
		v.serial = v.probeSerial(v.modbusUnitId, v.modbusClient, 2609)
		v.readModbusValues = v.readGridValues
	case domain.RolePv:
		v.model = "Victron PV"
		v.phases = v.probePhases(v.modbusUnitId, v.modbusClient, []uint16{1027, 1031, 1035})
		if v.phases == 0 {
			return fmt.Errorf("detected an unsupported %s meter (%v). Meter will not be queried for values", v.Brand(), v.role)
		}
		v.serial = v.probeSerial(v.modbusUnitId, v.modbusClient, 1039)
		v.readModbusValues = v.readPvValues
	case domain.RoleBattery:
		v.model = "Victron Battery"
		v.phases = 1
		_, err := v.modbusClient.ReadRegisters(225, 309, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err != nil {
			return fmt.Errorf("detected an unsupported %s meter (%v). Meter will not be queried for values", v.Brand(), v.role)
		}
		v.readModbusValues = v.readBatteryValues
	default:
		return fmt.Errorf("detected an unsupported %s meter (%v). Meter will not be queried for values", v.Brand(), v.role)
	}
	log.Infof("Detected a %d phase %s with unitId %d at %s.", v.phases, v.model, v.modbusUnitId, v.modbusClient.URL())
	if v.role != domain.RoleBattery {
		v.setDefaultLineIndices(fmt.Sprintf("%d phase %s %s with unitId %d at %s", v.phases, v.brand, v.model, v.modbusUnitId, v.modbusClient.URL()))
	}
	return nil
}

func (v *victronMeter) probePhases(modbusUnitId uint8, modbusClient *modbus.ModbusClient, addresses []uint16) uint8 {
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

func (v *victronMeter) probeSerial(modbusUnitId uint8, modbusClient *modbus.ModbusClient, address uint16) string {
	bytes, err := modbusClient.ReadBytes(modbusUnitId, address, 14, modbus.INPUT_REGISTER)
	if err != nil {
		log.Warningf("Unable to read %s serial: %s", v.Brand(), err.Error())
		return ""
	}
	return string(bytes)
}

func (v *victronMeter) readGridValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, _ *domain.BatteryState) error {
	modbusClient := v.modbusClient
	if electricityState != nil {
		if v.electricityMeter.HasPowerAttribute() {
			uint16s, err := modbusClient.ReadRegisters(v.modbusUnitId, 2600, 3, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			for ix := 0; ix < len(v.lineIndices); ix++ {
				electricityState.SetPower(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, v.lineIndices[ix], 0, 0))
			}
		}
		if v.electricityMeter.HasCurrentAttribute() || v.electricityMeter.HasVoltageAttribute() {
			uint16s, err := modbusClient.ReadRegisters(v.modbusUnitId, 2616, 6, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			for ix := 0; ix < len(v.lineIndices); ix++ {
				offset := v.lineIndices[ix] * 2
				if v.electricityMeter.HasVoltageAttribute() {
					electricityState.SetVoltage(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
				}
				if v.electricityMeter.HasCurrentAttribute() {
					electricityState.SetCurrent(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
				}
			}
		}
	}

	if electricityUsage != nil {
		if v.electricityMeter.HasConsumptionAttribute() {
			uint32s, err := modbusClient.ReadUint32s(v.modbusUnitId, 2622, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			for ix := 0; ix < len(v.lineIndices); ix++ {
				offset := v.lineIndices[ix]
				electricityUsage.SetEnergyConsumed(v.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, offset, 100, 0)))
			}
		}
		if v.electricityMeter.HasProductionAttribute() {
			uint32s, err := modbusClient.ReadUint32s(v.modbusUnitId, 2636, 1, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
			if err != nil {
				return err
			}
			if uint32s != nil {
				// Provided energy per phase is far from correct, so we split the total energy (which seems to be correct) equally over the given phases.
				provided := float64(modbusClient.ValueFromUint32sResultArray(uint32s, 0, 100, 0))
				providedPerPhase := provided / float64(len(v.lineIndices))
				for ix := 0; ix < len(v.lineIndices); ix++ {
					electricityUsage.SetEnergyProvided(v.lineIndices[ix], providedPerPhase)
				}
			}
		}
	}
	return nil
}

func (v *victronMeter) readPvValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, _ *domain.BatteryState) error {
	modbusClient := v.modbusClient
	if electricityState != nil && !(v.electricityMeter.HasVoltageAttribute() || v.electricityMeter.HasPowerAttribute() || v.electricityMeter.HasCurrentAttribute()) {
		uint16s, err := modbusClient.ReadRegisters(v.modbusUnitId, 1027, 11, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		for ix := 0; ix < len(v.lineIndices); ix++ {
			offset := v.lineIndices[ix] * 4
			if v.electricityMeter.HasVoltageAttribute() {
				electricityState.SetVoltage(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+0, 10, 0))
			}
			if v.electricityMeter.HasCurrentAttribute() {
				electricityState.SetCurrent(v.lineIndices[ix], modbusClient.ValueFromInt16sResultArray(uint16s, offset+1, 10, 0))
			}
			if v.electricityMeter.HasTotalPowerAttribute() {
				electricityState.SetPower(v.lineIndices[ix], modbusClient.ValueFromUint16sResultArray(uint16s, offset+2, 0, 0))
			}
		}
	}
	if electricityUsage != nil && v.electricityMeter.HasConsumptionAttribute() {
		uint32s, err := modbusClient.ReadUint32s(v.modbusUnitId, 1046, 3, modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST, modbus.INPUT_REGISTER)
		if err != nil {
			return err
		}
		if uint32s == nil || len(uint32s) < 3 {
			return nil
		}
		for ix := 0; ix < len(v.lineIndices); ix++ {
			electricityUsage.SetEnergyConsumed(v.lineIndices[ix], float64(modbusClient.ValueFromUint32sResultArray(uint32s, v.lineIndices[ix], 100, 0)))
		}
	}
	return nil
}

func (v *victronMeter) readBatteryValues(_ *domain.ElectricityState, _ *domain.ElectricityUsage, batteryState *domain.BatteryState) error {
	modbusClient := v.modbusClient
	uint16s, err := modbusClient.ReadRegisters(v.modbusUnitId, 258, 4, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	batteryState.SetVoltage(modbusClient.ValueFromUint16sResultArray(uint16s, 1, 100, 0))
	batteryState.SetCurrent(modbusClient.ValueFromInt16sResultArray(uint16s, 3, 10, 0))
	batteryState.SetPower(modbusClient.ValueFromInt16sResultArray(uint16s, 0, 0, 0))

	uint16s, err = modbusClient.ReadRegisters(v.modbusUnitId, 266, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	batteryState.SetSoC(modbusClient.ValueFromUint16sResultArray(uint16s, 0, 10, 0))

	uint16s, err = modbusClient.ReadRegisters(v.modbusUnitId, 304, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	batteryState.SetSoH(modbusClient.ValueFromUint16sResultArray(uint16s, 0, 10, 0))
	return nil
}

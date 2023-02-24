package proxy

import (
	"enman/internal/energysource"
	"enman/internal/modbus"
	"strconv"
)

type EM24MeterSimulator struct {
	modbus.RequestHandler
	energyFlow energysource.EnergyFlow
	unitId     uint8
}

func newEM24MeterSimulator(unitId uint8, energyFlow energysource.EnergyFlow) *EM24MeterSimulator {
	return &EM24MeterSimulator{
		energyFlow: energyFlow,
		unitId:     unitId,
	}
}

func (s *EM24MeterSimulator) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if req.IsWrite {
		return nil, modbus.ErrIllegalFunction
	} else {
		if req.Addr == 0x0000 && req.Quantity == 80 {
			var result = make([]uint16, 80)
			copy(result[0x0000:0x0002], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Voltage(0)*10)))
			copy(result[0x0002:0x0004], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Voltage(1)*10)))
			copy(result[0x0004:0x0006], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Voltage(2)*10)))

			copy(result[0x000c:0x000e], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Current(0)*1000)))
			copy(result[0x000e:0x0010], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Current(1)*1000)))
			copy(result[0x0010:0x0012], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Current(2)*1000)))

			copy(result[0x0012:0x0024], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Power(0)*10)))
			copy(result[0x0014:0x0016], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Power(1)*10)))
			copy(result[0x0016:0x0018], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.Power(2)*10)))
			copy(result[0x0028:0x002a], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.TotalPower()*10)))

			copy(result[0x0040:0x0042], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.EnergyConsumed(0)*10)))
			copy(result[0x0042:0x0044], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.EnergyConsumed(1)*10)))
			copy(result[0x0044:0x0046], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.EnergyConsumed(2)*10)))
			copy(result[0x0034:0x0036], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.TotalEnergyConsumed()*10)))
			copy(result[0x004e:], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.energyFlow.TotalEnergyProvided()*10)))
			return result, nil
		} else if req.Addr == 0x000b {
			return []uint16{0x0671}, nil
		} else if req.Addr == 0xa000 {
			return []uint16{0x0007}, nil
		} else if req.Addr == 0xa100 {
			return []uint16{0x0003}, nil
		} else if req.Addr == 0x0302 || req.Addr == 0x0304 {
			return []uint16{0x1000}, nil
		} else if req.Addr == 0x1002 {
			if s.energyFlow.Phases() == 3 {
				return []uint16{0x0000}, nil
			} else if s.energyFlow.Phases() == 2 {
				return []uint16{0x0002}, nil
			}
			return []uint16{0x0003}, nil
		} else if req.Addr == 0x5000 && req.Quantity == 7 {
			var result = make([]uint16, 7)
			serial := []byte{'E', 'n', 'M', 'a', 'n'}
			if s.unitId < 10 {
				serial = append(serial, '0')
			}
			serial = append(serial, []byte(strconv.Itoa(int(s.unitId)))...)
			for ix, b := range serial {
				if ix%2 == 0 {
					result[ix/2] = uint16(b) << 8
				} else {
					result[ix/2] += uint16(b)
				}
			}
			return result, nil
		}
		return nil, modbus.ErrIllegalDataAddress
	}
}

func (s *EM24MeterSimulator) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) uint32ToUint16s(endianness modbus.Endianness, wordOrder modbus.WordOrder, val uint32) []uint16 {
	bytes := modbus.Uint32ToBytes(endianness, wordOrder, val)
	return modbus.BytesToUint16s(endianness, bytes)
}

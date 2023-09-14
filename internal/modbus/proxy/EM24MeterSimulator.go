package proxy

import (
	"enman/internal/domain"
	"enman/internal/modbus"
	"strconv"
)

type EM24MeterSimulator struct {
	modbus.RequestHandler
	unitId           uint8
	electricityState *domain.ElectricityState
	electricityUsage *domain.ElectricityUsage
}

func newEM24MeterSimulator(unitId uint8, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) *EM24MeterSimulator {
	return &EM24MeterSimulator{
		unitId:           unitId,
		electricityState: electricityState,
		electricityUsage: electricityUsage,
	}
}

func (s *EM24MeterSimulator) HandleCoils(*modbus.CoilsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) HandleDiscreteInputs(*modbus.DiscreteInputsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if req.IsWrite {
		return nil, modbus.ErrIllegalFunction
	} else {

		var result = make([]uint16, req.Quantity)
		requestAddr := req.Addr
		resultAddr := uint16(0)
		for resultAddr < req.Quantity {
			length := uint16(1)
			switch requestAddr {
			case 0x0000:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Voltage(0)*10)))
			case 0x0002:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Voltage(1)*10)))
			case 0x0004:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Voltage(2)*10)))
			case 0x000b:
				copy(result[resultAddr:resultAddr+length], []uint16{0x0671})
			case 0x000c:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Current(0)*1000)))
			case 0x000e:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Current(1)*1000)))
			case 0x0010:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Current(2)*1000)))
			case 0x0012:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Power(0)*10)))
			case 0x0014:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Power(1)*10)))
			case 0x0016:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.Power(2)*10)))
			case 0x0028:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityState.TotalPower()*10)))
			case 0x0034:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityUsage.TotalEnergyConsumed()*10)))
			case 0x0040:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityUsage.EnergyConsumed(0)*10)))
			case 0x0042:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityUsage.EnergyConsumed(1)*10)))
			case 0x0044:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityUsage.EnergyConsumed(2)*10)))
			case 0x004e:
				length = 2
				copy(result[resultAddr:resultAddr+length], s.uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(s.electricityUsage.TotalEnergyProvided()*10)))
			case 0x0302, 0x0304:
				copy(result[resultAddr:resultAddr+length], []uint16{0x1000})
			case 0x1002:
				if s.electricityState.Phases() == 3 {
					copy(result[resultAddr:resultAddr+length], []uint16{0x0000})
				} else if s.electricityState.Phases() == 2 {
					copy(result[resultAddr:resultAddr+length], []uint16{0x0002})
				} else {
					copy(result[resultAddr:resultAddr+length], []uint16{0x0003})
				}
			case 0x5000:
				length = 7
				var serialResult = make([]uint16, length)
				serial := []byte{'E', 'n', 'M', 'a', 'n'}
				if s.unitId < 10 {
					serial = append(serial, '0')
				}
				serial = append(serial, []byte(strconv.Itoa(int(s.unitId)))...)
				for ix, b := range serial {
					if ix%2 == 0 {
						serialResult[ix/2] = uint16(b) << 8
					} else {
						serialResult[ix/2] += uint16(b)
					}
				}
				copy(result[resultAddr:resultAddr+length], serialResult)
			case 0xa000:
				copy(result[resultAddr:resultAddr+length], []uint16{0x0007})
			case 0xa100:
				copy(result[resultAddr:resultAddr+length], []uint16{0x0003})
			default:
			}
			requestAddr += length
			resultAddr += length
		}
		return result, nil
	}
}

func (s *EM24MeterSimulator) HandleInputRegisters(*modbus.InputRegistersRequest) ([]uint16, error) {
	return nil, modbus.ErrIllegalFunction
}

func (s *EM24MeterSimulator) uint32ToUint16s(endianness modbus.Endianness, wordOrder modbus.WordOrder, val uint32) []uint16 {
	bytes := modbus.Uint32ToBytes(endianness, wordOrder, val)
	return modbus.BytesToUint16s(endianness, bytes)
}

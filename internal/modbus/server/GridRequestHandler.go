package server

import (
	"enman/internal/domain"
	"enman/internal/modbus"
)

type GridRequestHandler struct {
	modbus.RequestHandler
	unitId uint8
	grid   *domain.Grid
}

func newGridRequestHandler(unitId uint8, grid *domain.Grid) *GridRequestHandler {
	return &GridRequestHandler{
		unitId: unitId,
		grid:   grid,
	}
}

func (g *GridRequestHandler) HandleCoils(*modbus.CoilsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (g *GridRequestHandler) HandleDiscreteInputs(*modbus.DiscreteInputsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}

func (g *GridRequestHandler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
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
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Voltage(0)*1000)))
			case 0x0002:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Voltage(1)*1000)))
			case 0x0004:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Voltage(2)*1000)))
			case 0x0010:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Current(0)*1000)))
			case 0x0012:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Current(1)*1000)))
			case 0x0014:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Current(2)*1000)))
			case 0x0016:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().TotalCurrent()*1000)))
			case 0x0020:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Power(0)*1000)))
			case 0x0022:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Power(1)*1000)))
			case 0x0024:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().Power(2)*1000)))
			case 0x0026:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityState().TotalPower()*1000)))
			case 0x0030:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyConsumed(0)*1000)))
			case 0x0032:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyConsumed(1)*1000)))
			case 0x0034:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyConsumed(2)*1000)))
			case 0x0036:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().TotalEnergyConsumed()*1000)))
			case 0x0040:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyProvided(0)*1000)))
			case 0x0042:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyProvided(1)*1000)))
			case 0x0044:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().EnergyProvided(2)*1000)))
			case 0x0046:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.ElectricityUsage().TotalEnergyProvided()*1000)))
			case 0x0050:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.GasUsage().GasConsumed()*1000)))
			case 0x0060:
				length = 2
				copy(result[resultAddr:resultAddr+length], modbus.Uint32ToUint16s(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST, uint32(g.grid.WaterUsage().WaterConsumed()*1000)))
			default:
			}
			requestAddr += length
			resultAddr += length
		}
		return result, nil
	}
}

func (g *GridRequestHandler) HandleInputRegisters(*modbus.InputRegistersRequest) ([]uint16, error) {
	return nil, modbus.ErrIllegalFunction
}

package server

import (
	"enman/internal/domain"
	"enman/internal/domain/electricity"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
)

type DispatchingRequestHandler struct {
	modbus.RequestHandler
	unitIdMapping map[uint8]modbus.RequestHandler
}

func NewDispatchingRequestHandler(system *domain.System) *DispatchingRequestHandler {
	drh := &DispatchingRequestHandler{
		unitIdMapping: make(map[uint8]modbus.RequestHandler),
	}
	drh.unitIdMapping[1] = newGridRequestHandler(1, system.Grid())

	return drh
}

func NewMeterSimulator(meterType string, unitId uint8, electricityState *electricity.State, electricityUsage *electricity.Usage) (modbus.RequestHandler, error) {
	if unitId < 100 {
		return nil, fmt.Errorf("meter simulator must have a unit id >= 100")
	}
	switch meterType {
	case "EM24":
		return newEM24MeterSimulator(unitId, electricityState, electricityUsage), nil
	default:
		return nil, fmt.Errorf("unknown meter simulator type '%s'", meterType)
	}
}

func (h *DispatchingRequestHandler) HandleCoils(req *modbus.CoilsRequest) ([]bool, error) {
	handler, available := h.unitIdMapping[req.UnitId]
	if !available {
		return nil, modbus.ErrIllegalDataAddress
	}
	return handler.HandleCoils(req)
}

func (h *DispatchingRequestHandler) HandleDiscreteInputs(req *modbus.DiscreteInputsRequest) ([]bool, error) {
	handler, available := h.unitIdMapping[req.UnitId]
	if !available {
		return nil, modbus.ErrIllegalDataAddress
	}
	return handler.HandleDiscreteInputs(req)
}

func (h *DispatchingRequestHandler) HandleHoldingRegisters(req *modbus.HoldingRegistersRequest) ([]uint16, error) {
	handler, available := h.unitIdMapping[req.UnitId]
	if !available {
		return nil, modbus.ErrIllegalDataAddress
	}
	log.Tracef("Received holding request from %s for unitId %d at address %#04x and quantity %d", req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return handler.HandleHoldingRegisters(req)
}

func (h *DispatchingRequestHandler) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	handler, available := h.unitIdMapping[req.UnitId]
	if !available {
		return nil, modbus.ErrIllegalDataAddress
	}
	log.Tracef("Received input request from %s for unitId %d at address %#04x and quantity %d", req.ClientAddr, req.UnitId, req.Addr, req.Quantity)
	return handler.HandleInputRegisters(req)
}

func (h *DispatchingRequestHandler) AddHandler(unitId uint8, handler modbus.RequestHandler) {
	h.unitIdMapping[unitId] = handler
}

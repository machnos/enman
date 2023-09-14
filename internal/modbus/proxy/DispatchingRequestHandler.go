package proxy

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
)

type DispatchingRequestHandler struct {
	modbus.RequestHandler
	unitIdMapping map[uint8]modbus.RequestHandler
}

func NewDispatchingRequestHandler() *DispatchingRequestHandler {
	return &DispatchingRequestHandler{
		unitIdMapping: make(map[uint8]modbus.RequestHandler),
	}
}

func NewMeterSimulator(meterType string, unitId uint8, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) modbus.RequestHandler {
	switch meterType {
	case "EM24":
		return newEM24MeterSimulator(unitId, electricityState, electricityUsage)
	default:
		log.Warningf("Unknown meter simulator type '%s'", meterType)
	}
	return nil
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
	return handler.HandleHoldingRegisters(req)
}

func (h *DispatchingRequestHandler) HandleInputRegisters(req *modbus.InputRegistersRequest) ([]uint16, error) {
	handler, available := h.unitIdMapping[req.UnitId]
	if !available {
		return nil, modbus.ErrIllegalDataAddress
	}
	return handler.HandleInputRegisters(req)
}

func (h *DispatchingRequestHandler) AddHandler(unitId uint8, handler modbus.RequestHandler) {
	h.unitIdMapping[unitId] = handler
}

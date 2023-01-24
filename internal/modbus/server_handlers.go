package modbus

type proxyRequestHandler struct {
	RequestHandler
	client *ModbusClient
}

func NewProxyRequestHandler(client *ModbusClient) *proxyRequestHandler {
	return &proxyRequestHandler{
		client: client,
	}
}

func (p *proxyRequestHandler) HandleCoils(req *CoilsRequest) ([]bool, error) {
	println("Received coilRequest")
	p.client.SetUnitId(req.UnitId)
	if req.IsWrite {
		return nil, p.client.WriteCoils(req.Addr, req.Args)
	} else {
		return p.client.ReadCoils(req.Addr, req.Quantity)
	}
}

func (p *proxyRequestHandler) HandleDiscreteInputs(req *DiscreteInputsRequest) ([]bool, error) {
	println("Received ReadDiscreteInputs")
	p.client.SetUnitId(req.UnitId)
	return p.client.ReadDiscreteInputs(req.Addr, req.Quantity)
}

func (p *proxyRequestHandler) HandleHoldingRegisters(req *HoldingRegistersRequest) (res []uint16, err error) {
	println("Received HandleHoldingRegisters")
	p.client.SetUnitId(req.UnitId)
	if req.IsWrite {
		return nil, p.client.WriteRegisters(req.Addr, req.Args)
	} else {
		return p.client.ReadRegisters(req.Addr, req.Quantity, HOLDING_REGISTER)
	}
}

func (p *proxyRequestHandler) HandleInputRegisters(req *InputRegistersRequest) (res []uint16, err error) {
	println("Received HandleInputRegisters")
	p.client.SetUnitId(req.UnitId)
	return p.client.ReadRegisters(req.Addr, req.Quantity, INPUT_REGISTER)
}

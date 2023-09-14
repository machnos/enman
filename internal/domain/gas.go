package domain

type GasUsage struct {
	gasConsumed float64
}

func NewGasUsage() *GasUsage {
	return &GasUsage{}
}

func (gu *GasUsage) GasConsumed() float64 {
	return gu.gasConsumed
}

package domain

type WaterUsage struct {
	waterConsumed float64
}

func NewWaterUsage() *WaterUsage {
	return &WaterUsage{}
}

func (wu *WaterUsage) WaterConsumed() float64 {
	return wu.waterConsumed
}

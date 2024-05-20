package domain

type ElectricityMeter interface {
	HasStateAttribute() bool
	HasUsageAttribute() bool
	LineIndices() []uint8
	Phases() uint8
}

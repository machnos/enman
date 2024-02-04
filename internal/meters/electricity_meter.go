package meters

import (
	"enman/internal/config"
	"slices"
)

type electricityMeter struct {
	attributes  []string
	lineIndices []uint8
	phases      uint8
}

func newElectricityMeter(config *config.EnergyMeter) *electricityMeter {
	return &electricityMeter{
		lineIndices: config.LineIndices,
		attributes:  config.Attributes,
	}
}

func (e *electricityMeter) HasStateAttribute() bool {
	return len(e.attributes) == 0 || slices.Contains(e.attributes, "state")
}
func (e *electricityMeter) HasUsageAttribute() bool {
	return len(e.attributes) == 0 || slices.Contains(e.attributes, "usage")
}
func (e *electricityMeter) LineIndices() []uint8 {
	return e.lineIndices
}
func (e *electricityMeter) Phases() uint8 {
	return e.phases
}

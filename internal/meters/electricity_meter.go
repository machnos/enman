package meters

import (
	"enman/internal/config"
	"enman/internal/log"
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

func (e *electricityMeter) setDefaultLineIndices(meterIdentification string) {
	if e.lineIndices == nil {
		if e.phases == 1 {
			log.Infof("%s has no configured line indices. If this meter is used in a multi phase system it is assumed this meter will read values for L1.", meterIdentification)
			e.lineIndices = []uint8{0}
		} else if e.phases == 2 {
			log.Infof("%s has no configured line indices. If this meter is used in a multi phase system it is assumed this meter will read values for L1 & L2.", meterIdentification)
			e.lineIndices = []uint8{0, 1}
		} else if e.phases == 3 {
			log.Infof("%s has no configured line indices. If this meter is used in a multi phase system it is assumed this meter will read values for L1, L2 & L3.", meterIdentification)
			e.lineIndices = []uint8{0, 1, 2}
		}
	}
}

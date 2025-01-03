package meters

import (
	"enman/internal/config"
	"enman/internal/domain/electricity"
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

func (e *electricityMeter) HasAttribute(attribute electricity.MeterAttribute) bool {
	return len(e.attributes) == 0 || slices.Contains(e.attributes, string(attribute))
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

func (e *electricityMeter) HasCurrentAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeCurrent) || e.HasAttribute(electricity.MeterAttributeAllstate)
}
func (e *electricityMeter) HasTotalCurrentAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeTotalCurrent) || e.HasAttribute(electricity.MeterAttributeAllstate)
}
func (e *electricityMeter) HasPowerAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributePower) || e.HasAttribute(electricity.MeterAttributeAllstate)
}
func (e *electricityMeter) HasTotalPowerAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeTotalPower) || e.HasAttribute(electricity.MeterAttributeAllstate)
}
func (e *electricityMeter) HasVoltageAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeVoltage) || e.HasAttribute(electricity.MeterAttributeAllstate)
}
func (e *electricityMeter) HasConsumptionAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeConsumption) || e.HasAttribute(electricity.MeterAttributeAllUsage)
}
func (e *electricityMeter) HasTotalConsumptionAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeTotalConsumption) || e.HasAttribute(electricity.MeterAttributeAllUsage)
}
func (e *electricityMeter) HasProductionAttribute() bool {
	return e.HasAttribute(electricity.MeterAttributeProduction) || e.HasAttribute(electricity.MeterAttributeAllUsage)
}
func (e *electricityMeter) HasTotalProductionAttribute() bool {
	return e.HasAttribute(electricity.ElectricityMeterAttributeTotalProduction) || e.HasAttribute(electricity.MeterAttributeAllUsage)
}

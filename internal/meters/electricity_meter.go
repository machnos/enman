package meters

import (
	"enman/internal/config"
	"enman/internal/domain"
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

func (e *electricityMeter) HasAttribute(attribute domain.ElectricityMeterAttribute) bool {
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
	return e.HasAttribute(domain.ElectricityMeterAttributeCurrent) || e.HasAttribute(domain.ElectricityMeterAttributeAllstate)
}
func (e *electricityMeter) HasTotalCurrentAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeTotalCurrent) || e.HasAttribute(domain.ElectricityMeterAttributeAllstate)
}
func (e *electricityMeter) HasPowerAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributePower) || e.HasAttribute(domain.ElectricityMeterAttributeAllstate)
}
func (e *electricityMeter) HasTotalPowerAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeTotalPower) || e.HasAttribute(domain.ElectricityMeterAttributeAllstate)
}
func (e *electricityMeter) HasVoltageAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeVoltage) || e.HasAttribute(domain.ElectricityMeterAttributeAllstate)
}
func (e *electricityMeter) HasConsumptionAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeConsumption) || e.HasAttribute(domain.ElectricityMeterAttributeAllUsage)
}
func (e *electricityMeter) HasTotalConsumptionAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeTotalConsumption) || e.HasAttribute(domain.ElectricityMeterAttributeAllUsage)
}
func (e *electricityMeter) HasProductionAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeProduction) || e.HasAttribute(domain.ElectricityMeterAttributeAllUsage)
}
func (e *electricityMeter) HasTotalProductionAttribute() bool {
	return e.HasAttribute(domain.ElectricityMeterAttributeTotalProduction) || e.HasAttribute(domain.ElectricityMeterAttributeAllUsage)
}

package domain

type ElectricityMeterAttribute string

const (
	ElectricityMeterAttributeAllstate         ElectricityMeterAttribute = "state"
	ElectricityMeterAttributeCurrent          ElectricityMeterAttribute = "current"
	ElectricityMeterAttributeTotalCurrent     ElectricityMeterAttribute = "total_current"
	ElectricityMeterAttributePower            ElectricityMeterAttribute = "power"
	ElectricityMeterAttributeTotalPower       ElectricityMeterAttribute = "total_power"
	ElectricityMeterAttributeVoltage          ElectricityMeterAttribute = "voltage"
	ElectricityMeterAttributeAllUsage         ElectricityMeterAttribute = "usage"
	ElectricityMeterAttributeConsumption      ElectricityMeterAttribute = "consumption"
	ElectricityMeterAttributeTotalConsumption ElectricityMeterAttribute = "total_consumption"
	ElectricityMeterAttributeProduction       ElectricityMeterAttribute = "production"
	ElectricityMeterAttributeTotalProduction  ElectricityMeterAttribute = "total_production"
)

type ElectricityMeter interface {
	HasAttribute(attribute ElectricityMeterAttribute) bool
	LineIndices() []uint8
	Phases() uint8
}

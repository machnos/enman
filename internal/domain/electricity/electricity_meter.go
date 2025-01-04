package electricity

import "time"

type MeterAttribute string

const (
	MeterAttributeAllstate         MeterAttribute = "state"
	MeterAttributeCurrent          MeterAttribute = "current"
	MeterAttributeTotalCurrent     MeterAttribute = "total_current"
	MeterAttributePower            MeterAttribute = "power"
	MeterAttributeTotalPower       MeterAttribute = "total_power"
	MeterAttributeVoltage          MeterAttribute = "voltage"
	MeterAttributeAllUsage         MeterAttribute = "usage"
	MeterAttributeConsumption      MeterAttribute = "consumption"
	MeterAttributeTotalConsumption MeterAttribute = "total_consumption"
	MeterAttributeProduction       MeterAttribute = "production"
	MeterAttributeTotalProduction  MeterAttribute = "total_production"

	MeterUsageUpdateInterval = time.Second * 10
)

type Meter interface {
	HasAttribute(attribute MeterAttribute) bool
	LineIndices() []uint8
	Phases() uint8
}

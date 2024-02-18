package domain

import "time"

type EnergyMeter interface {
	Brand() string
	Model() string
	Serial() string
	UpdateInterval() time.Duration
	UpdateValues(
		electricityState *ElectricityState,
		electricityUsage *ElectricityUsage,
		gasUsage *GasUsage,
		waterUsage *WaterUsage,
		batteryState *BatteryState)
	Shutdown()
}

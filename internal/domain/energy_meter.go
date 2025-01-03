package domain

import (
	"enman/internal/domain/battery"
	"enman/internal/domain/electricity"
	"enman/internal/domain/gas"
	"enman/internal/domain/water"
	"time"
)

type EnergyMeter interface {
	Brand() string
	Model() string
	Serial() string
	UpdateInterval() time.Duration
	UpdateValues(
		electricityState *electricity.State,
		electricityUsage *electricity.Usage,
		gasUsage *gas.Usage,
		waterUsage *water.Usage,
		batteryState *battery.State) error
	Shutdown()
}

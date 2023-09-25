package domain

import "context"

type EnergyMeter interface {
	Name() string
	Role() EnergySourceRole
	Brand() string
	Model() string
	Serial() string
	StartReading(context.Context)
}

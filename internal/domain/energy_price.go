package domain

import "time"

type EnergyPrice struct {
	Time             time.Time
	ConsumptionPrice float32
	FeedbackPrice    float32
	Provider         string
}

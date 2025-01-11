package repository

import (
	"enman/internal/domain/constants"
	"time"
)

type Water interface {
	WaterSourceNames(from time.Time, till time.Time) ([]string, error)
	WaterUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*WaterUsagesRecord, error)
	WaterUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*WaterUsageRecord, error)
}

package repository

import (
	"enman/internal/domain/constants"
	"time"
)

type Battery interface {
	BatterySourceNames(from time.Time, till time.Time) ([]string, error)
	BatteryStates(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*BatteryStatesRecord, error)
	BatteryStateAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*BatteryStateRecord, error)
}

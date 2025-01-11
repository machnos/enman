package repository

import (
	"enman/internal/domain/constants"
	"time"
)

type Gas interface {
	GasSourceNames(from time.Time, till time.Time) ([]string, error)
	GasUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*GasUsagesRecord, error)
	GasUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*GasUsageRecord, error)
	GasCosts(from time.Time, till time.Time, providerName string, aggregate *AggregateConfiguration) ([]*GasCostsRecord, error)
}

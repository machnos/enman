package repository

import (
	"enman/internal/domain/constants"
	"time"
)

type Electricity interface {
	ElectricitySourceNames(from time.Time, till time.Time) ([]string, error)
	ElectricityUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityUsagesRecord, error)
	ElectricityUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*ElectricityUsageRecord, error)
	ElectricityStates(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityStatesRecord, error)
	ElectricityCosts(from time.Time, till time.Time, providerName string, aggregate *AggregateConfiguration) ([]*ElectricityCostsRecord, error)
}

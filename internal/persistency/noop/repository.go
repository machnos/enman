package noop

import (
	"enman/internal/domain/constants"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"time"
)

type Repository struct {
	repository.Repository
}

func (r *Repository) ElectricitySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsages(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.ElectricityUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ repository.MatchType) (*repository.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityStates(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.ElectricityStatesRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityCosts(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.ElectricityCostsRecord, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceProviders(_ time.Time, _ time.Time) ([]*prices.EnergyPriceProvider, error) {
	return nil, nil
}
func (r *Repository) EnergyPrices(_ time.Time, _ time.Time, _ string, _ prices.EnergyType) ([]*prices.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceAtTime(_ time.Time, _ string, _ prices.EnergyType, _ repository.MatchType) (*prices.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) StoreEnergyPrice(_ *prices.EnergyPrice) error {
	return nil
}
func (r *Repository) GasSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) GasUsages(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.GasUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) GasUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ repository.MatchType) (*repository.GasUsageRecord, error) {
	return nil, nil
}
func (r *Repository) WaterSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) WaterUsages(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.WaterUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) WaterUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ repository.MatchType) (*repository.WaterUsageRecord, error) {
	return nil, nil
}
func (r *Repository) BatterySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) BatteryStates(_ time.Time, _ time.Time, _ string, _ *repository.AggregateConfiguration) ([]*repository.BatteryStatesRecord, error) {
	return nil, nil
}
func (r *Repository) BatteryStateAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ repository.MatchType) (*repository.BatteryStateRecord, error) {
	return nil, nil
}

func (r *Repository) Close() {
}
func (r *Repository) Initialize() error {
	return nil
}
func NewNoopRepository() *Repository {
	return &Repository{}
}

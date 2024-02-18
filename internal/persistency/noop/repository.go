package noop

import (
	"enman/internal/domain"
	"time"
)

type Repository struct {
	domain.Repository
}

func (r *Repository) ElectricitySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsageAtTime(_ time.Time, _ string, _ domain.EnergySourceRole, _ domain.MatchType) (*domain.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityStates(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityStateRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityCosts(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityCostRecord, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceProviderNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) EnergyPrices(_ time.Time, _ time.Time, _ string) ([]*domain.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceAtTime(_ time.Time, _ string, _ domain.MatchType) (*domain.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) StoreEnergyPrice(_ *domain.EnergyPrice) {
}
func (r *Repository) GasSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) GasUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.GasUsageRecord, error) {
	return nil, nil
}
func (r *Repository) GasUsageAtTime(_ time.Time, _ string, _ domain.EnergySourceRole, _ domain.MatchType) (*domain.GasUsageRecord, error) {
	return nil, nil
}
func (r *Repository) WaterSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) WaterUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.WaterUsageRecord, error) {
	return nil, nil
}
func (r *Repository) WaterUsageAtTime(_ time.Time, _ string, _ domain.EnergySourceRole, _ domain.MatchType) (*domain.WaterUsageRecord, error) {
	return nil, nil
}
func (r *Repository) BatterySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) BatteryStates(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.BatteryStateRecord, error) {
	return nil, nil
}
func (r *Repository) BatteryStateAtTime(_ time.Time, _ string, _ domain.EnergySourceRole, _ domain.MatchType) (*domain.BatteryStateRecord, error) {
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

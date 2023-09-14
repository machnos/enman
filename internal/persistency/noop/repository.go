package noop

import (
	"enman/internal/domain"
	"time"
)

type Repository struct {
	domain.Repository
}

func (r *Repository) ElectricitySourceNames(time.Time, time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsages(time.Time, time.Time, string, *domain.AggregateConfiguration) ([]*domain.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsageAtTime(time.Time, string, domain.ElectricitySourceRole, domain.MatchType) (*domain.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityStates(time.Time, time.Time, string, *domain.AggregateConfiguration) ([]*domain.ElectricityStateRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityCosts(time.Time, time.Time, string, *domain.AggregateConfiguration) ([]*domain.ElectricityCostRecord, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceProviderNames(time.Time, time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) EnergyPrices(time.Time, time.Time, string) ([]*domain.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceAtTime(time.Time, string, domain.MatchType) (*domain.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) StoreEnergyPrice(*domain.EnergyPrice) {
}
func (r *Repository) Close() {
}
func (r *Repository) Initialize() error {
	return nil
}
func NewNoopRepository() *Repository {
	return &Repository{}
}

package noop

import (
	"enman/internal/domain"
	"enman/internal/domain/constants"
	"enman/internal/domain/prices"
	"time"
)

type Repository struct {
	domain.Repository
}

func (r *Repository) ElectricitySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ domain.MatchType) (*domain.ElectricityUsageRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityStates(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityStatesRecord, error) {
	return nil, nil
}
func (r *Repository) ElectricityCosts(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.ElectricityCostsRecord, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceProviders(_ time.Time, _ time.Time) ([]*prices.EnergyPriceProvider, error) {
	return nil, nil
}
func (r *Repository) EnergyPrices(_ time.Time, _ time.Time, _ string, _ prices.EnergyType) ([]*prices.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) EnergyPriceAtTime(_ time.Time, _ string, _ prices.EnergyType, _ domain.MatchType) (*prices.EnergyPrice, error) {
	return nil, nil
}
func (r *Repository) StoreEnergyPrice(_ *prices.EnergyPrice) error {
	return nil
}
func (r *Repository) GasSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) GasUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.GasUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) GasUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ domain.MatchType) (*domain.GasUsageRecord, error) {
	return nil, nil
}
func (r *Repository) WaterSourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) WaterUsages(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.WaterUsagesRecord, error) {
	return nil, nil
}
func (r *Repository) WaterUsageAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ domain.MatchType) (*domain.WaterUsageRecord, error) {
	return nil, nil
}
func (r *Repository) BatterySourceNames(_ time.Time, _ time.Time) ([]string, error) {
	return nil, nil
}
func (r *Repository) BatteryStates(_ time.Time, _ time.Time, _ string, _ *domain.AggregateConfiguration) ([]*domain.BatteryStatesRecord, error) {
	return nil, nil
}
func (r *Repository) BatteryStateAtTime(_ time.Time, _ string, _ constants.EnergySourceRole, _ domain.MatchType) (*domain.BatteryStateRecord, error) {
	return nil, nil
}
func (r *Repository) StoreForecast(_ *domain.ForecastRecord) error {
	return nil
}
func (r *Repository) Forecasts(_ time.Time, _ time.Time, _ string, _ string) ([]*domain.ForecastRecord, error) {
	return nil, nil
}
func (r *Repository) ForecastAtTime(_ time.Time, _ string, _ string, _ domain.MatchType) (*domain.ForecastRecord, error) {
	return nil, nil
}
func (r *Repository) StoreBatterySchedule(_ *domain.BatteryScheduleRecord) error {
	return nil
}
func (r *Repository) BatterySchedules(_ time.Time, _ time.Time, _ string) ([]*domain.BatteryScheduleRecord, error) {
	return nil, nil
}
func (r *Repository) BatteryScheduleAtTime(_ time.Time, _ string, _ domain.MatchType) (*domain.BatteryScheduleRecord, error) {
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

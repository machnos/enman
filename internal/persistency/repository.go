package persistency

import (
	"enman/internal/energysource"
	"enman/internal/prices"
	"time"
)

type Repository interface {
	EnergyFlowNames() ([]string, error)
	EnergyFlowUsages(from *time.Time, till *time.Time, name string, aggregate *AggregateConfiguration) ([]*EnergyFlowUsage, error)
	EnergyFlowStates(from *time.Time, till *time.Time, name string, aggregate *AggregateConfiguration) ([]*EnergyFlowState, error)
	StoreEnergyFlow(flow energysource.EnergyFlow)

	EnergyPriceProviders() ([]string, error)
	EnergyPrices(from *time.Time, till *time.Time, provider string) ([]*prices.EnergyPrice, error)
	StoreEnergyPrice(price *prices.EnergyPrice)

	Initialize() error
	Close()
}

type EnergyFlowUsage struct {
	Time time.Time
	Name string
	Role string
	*energysource.EnergyFlowUsage
}

type EnergyFlowState struct {
	Time time.Time
	Name string
	Role string
	*energysource.EnergyFlowState
}

type NoopRepository struct {
	Repository
}

func (n *NoopRepository) EnergyFlowNames() ([]string, error) {
	return nil, nil
}
func (n *NoopRepository) EnergyFlowUsages(*time.Time, *time.Time, string, *AggregateConfiguration) ([]*EnergyFlowUsage, error) {
	return nil, nil
}
func (n *NoopRepository) EnergyFlowStates(*time.Time, *time.Time, string, *AggregateConfiguration) ([]*EnergyFlowState, error) {
	return nil, nil
}
func (n *NoopRepository) StoreEnergyFlow(energysource.EnergyFlow) {
}
func (n *NoopRepository) EnergyPriceProviders() ([]string, error) {
	return nil, nil
}
func (n *NoopRepository) EnergyPrices(*time.Time, *time.Time, string) ([]*prices.EnergyPrice, error) {
	return nil, nil
}
func (n *NoopRepository) StoreEnergyPrice(*prices.EnergyPrice) {
}
func (n *NoopRepository) Close() {
}
func (n *NoopRepository) Initialize() error {
	return nil
}
func NewNoopRepository() *NoopRepository {
	return &NoopRepository{}
}

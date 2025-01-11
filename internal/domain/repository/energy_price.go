package repository

import (
	"enman/internal/domain/prices"
	"time"
)

type EnergyPrice interface {
	EnergyPriceProviders(from time.Time, till time.Time) ([]*prices.EnergyPriceProvider, error)
	EnergyPrices(from time.Time, till time.Time, providerName string, energyType prices.EnergyType) ([]*prices.EnergyPrice, error)
	EnergyPriceAtTime(moment time.Time, providerName string, energyType prices.EnergyType, timeMatchType MatchType) (*prices.EnergyPrice, error)
	StoreEnergyPrice(price *prices.EnergyPrice) error
}

package timescale

import (
	"context"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"enman/internal/persistency/sql"
	"fmt"
	"time"
)

const (
	tableEnergyPriceProviders = "energy_price_providers"
	tableEnergyPrices         = "energy_prices"
)

func (t *timescaleRepository) newEnergyProvidersDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableEnergyPriceProviders,
		Columns: []*sql.ColumnDefinition{
			{"name", "VARCHAR(50)", false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newEnergyPricesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableEnergyPrices,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"provider", "VARCHAR(50)", false},
			{"energy_type", "VARCHAR(20)", false},
			{"consumption_price", "DOUBLE PRECISION", true},
			{"feedback_price", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "provider", "energy_type"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"provider"}, tableEnergyPriceProviders, []string{"name"}},
		},
	}
}

func (t *timescaleRepository) EnergyPriceProviders(from time.Time, till time.Time) ([]*prices.EnergyPriceProvider, error) {
	statement, err := sql.NewSelect(tableEnergyPrices).
		Distinct().
		WithColumns(sql.NewColumns("provider", "energy_type")...).
		WithFilter(t.timeRangeFilter("time", from, till)).
		OrderAscending(sql.NewColumnWithName("provider")).
		Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*prices.EnergyPriceProvider
	currentProvider := &prices.EnergyPriceProvider{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		name := values[0].(string)
		energyType, err := prices.ParseEnergyType(values[1].(string))
		if err != nil {
			return nil, err
		}
		if name != currentProvider.Name {
			if currentProvider.Name != "" {
				providers = append(providers, currentProvider)
			}
			currentProvider = &prices.EnergyPriceProvider{
				Name: name,
			}
		}
		currentProvider.EnergyTypes = append(currentProvider.EnergyTypes, energyType)
	}
	if currentProvider.Name != "" {
		providers = append(providers, currentProvider)
	}
	return providers, nil
}

func (t *timescaleRepository) EnergyPrices(from time.Time, till time.Time, providerName string, energyType prices.EnergyType) ([]*prices.EnergyPrice, error) {
	tdPrices := t.tableDefinitions[tableEnergyPrices]

	filter := t.timeRangeFilter("time", from, till)
	if providerName != "" {
		filter.And("provider", sql.Equals, providerName)
	}
	if energyType != prices.EnergyTypeNone {
		filter.And("energy_type", sql.Equals, energyType.String())
	}

	statement, err := sql.NewSelect(tableEnergyPrices).
		WithColumns(sql.NewColumns(tdPrices.ColumnNames()...)...).
		WithFilter(filter).
		Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	energyPrices := make([]*prices.EnergyPrice, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		energyPrices = append(energyPrices, t.rowValuesToEnergyPrice(values))
	}
	return energyPrices, nil
}

func (t *timescaleRepository) EnergyPriceAtTime(moment time.Time, providerName string, energyType prices.EnergyType, timeMatchType repository.MatchType) (*prices.EnergyPrice, error) {
	tdPrices := t.tableDefinitions[tableEnergyPrices]

	filter := t.momentFilter("time", moment, timeMatchType)
	if providerName != "" {
		filter.And("provider", sql.Equals, providerName)
	}
	if energyType != prices.EnergyTypeNone {
		filter.And("energy_type", sql.Equals, energyType.String())
	}
	selectStatement := sql.NewSelect(tableEnergyPrices).
		WithColumns(sql.NewColumns(tdPrices.ColumnNames()...)...).
		WithFilter(filter)

	switch timeMatchType {
	case repository.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName("time"))
		break
	case repository.EqualOrGreater:
		selectStatement.OrderAscending(sql.NewColumnWithName("time"))
		break
	default:
		break
	}
	statement, err := selectStatement.Build()
	if err != nil {
		return nil, err
	}

	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}
	values, err := rows.Values()
	if err != nil {
		return nil, err
	}
	return t.rowValuesToEnergyPrice(values), nil
}

func (t *timescaleRepository) StoreEnergyPrice(price *prices.EnergyPrice) error {
	t.registerEnergyPriceProvider(price.ProviderName)
	_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableEnergyPrices],
		price.Time,
		price.ProviderName,
		price.EnergyType.String(),
		price.ConsumptionPrice,
		price.FeedbackPrice,
	)
	return err
}

func (t *timescaleRepository) rowValuesToEnergyPrice(values []any) *prices.EnergyPrice {
	energyType, err := prices.ParseEnergyType(values[2].(string))
	if err != nil {
		log.Warning(err.Error())
	}
	return &prices.EnergyPrice{
		Time:             values[0].(time.Time),
		ProviderName:     values[1].(string),
		EnergyType:       energyType,
		ConsumptionPrice: float32(values[3].(float64)),
		FeedbackPrice:    float32(values[4].(float64)),
	}
}

func (t *timescaleRepository) registerEnergyPriceProvider(provider string) {
	cacheKey := fmt.Sprintf("energy-price-provider-%s", provider)
	_, ok := t.energySourcesCache[cacheKey]
	if !ok {
		_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableEnergyPriceProviders], provider)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("failed to register energy price provider '%s': %v", provider, err)
			}
			return
		}
		t.energySourcesCache[cacheKey] = true
	}
}

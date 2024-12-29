package timescale

import (
	"context"
	"enman/internal/domain"
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
			{"consumption_price", "DOUBLE PRECISION", true},
			{"feedback_price", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "provider"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"provider"}, tableEnergyPriceProviders, []string{"name"}},
		},
	}
}

func (t *timescaleRepository) EnergyPriceProviderNames(from time.Time, till time.Time) ([]string, error) {
	statement, err := sql.NewSelect(tableEnergyPrices).
		WithColumns(sql.NewDistinctColumn("provider")).
		WithFilter(t.timeRangeFilter("time", from, till)).
		Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (t *timescaleRepository) EnergyPrices(from time.Time, till time.Time, providerName string) ([]*domain.EnergyPrice, error) {
	tdPrices := t.tableDefinitions[tableEnergyPrices]

	filter := t.timeRangeFilter("time", from, till)
	if providerName != "" {
		filter.And("provider", sql.Equals, providerName)
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

	energyPrices := make([]*domain.EnergyPrice, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		energyPrices = append(energyPrices, t.rowValuesToEnergyPrice(values))
	}
	return energyPrices, nil
}

func (t *timescaleRepository) EnergyPriceAtTime(moment time.Time, providerName string, timeMatchType domain.MatchType) (*domain.EnergyPrice, error) {
	tdPrices := t.tableDefinitions[tableEnergyPrices]

	filter := t.momentFilter("time", moment, timeMatchType)
	if providerName != "" {
		filter.And("provider", sql.Equals, providerName)
	}
	selectStatement := sql.NewSelect(tableEnergyPrices).
		WithColumns(sql.NewColumns(tdPrices.ColumnNames()...)...).
		WithFilter(filter)

	switch timeMatchType {
	case domain.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName("time"))
		break
	case domain.EqualOrGreater:
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

func (t *timescaleRepository) StoreEnergyPrice(price *domain.EnergyPrice) error {
	t.registerEnergyPriceProvider(price.Provider)
	_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableEnergyPrices], price.Time, price.Provider, price.ConsumptionPrice, price.FeedbackPrice)
	return err
}

func (t *timescaleRepository) rowValuesToEnergyPrice(values []any) *domain.EnergyPrice {
	return &domain.EnergyPrice{
		Time:             values[0].(time.Time),
		Provider:         values[1].(string),
		ConsumptionPrice: float32(values[2].(float64)),
		FeedbackPrice:    float32(values[3].(float64)),
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

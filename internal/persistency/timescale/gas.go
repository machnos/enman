package timescale

import (
	"context"
	"enman/internal/domain"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/gas"
	"enman/internal/log"
	"enman/internal/persistency/sql"
	"fmt"
	"time"
)

const (
	tableGasSources = "gas_sources"
	tableGasUsages  = "gas_usages"
)

func (t *timescaleRepository) newGasSourcesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableGasSources,
		Columns: []*sql.ColumnDefinition{
			{"name", "VARCHAR(50)", false},
			{"role", "VARCHAR(25)", false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newGasUsagesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableGasUsages,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"gas_source", "VARCHAR(50)", false},
			{"gas_consumed", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "gas_source"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"gas_source"}, tableGasSources, []string{"name"}},
		},
	}
	return td
}

func (t *timescaleRepository) GasSourceNames(from time.Time, till time.Time) ([]string, error) {
	filter := t.timeRangeFilter("time", from, till)
	statement, err := sql.NewSelect(tableGasUsages).
		WithColumns(sql.NewDistinctColumn("gas_source")).
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

func (t *timescaleRepository) GasUsages(from time.Time, till time.Time, sourceName string, aggregate *domain.AggregateConfiguration) ([]*domain.GasUsagesRecord, error) {
	tdUsages := t.tableDefinitions[tableGasUsages]
	tdSources := t.tableDefinitions[tableGasSources]

	filter := t.timeRangeFilter(tdUsages.TablePrefixedColumn("time"), from, till)
	if sourceName != "" {
		filter.And(tdSources.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}

	aggregateColumn := t.toAggregateWindowColumn(tdUsages.TablePrefixedColumn("time"), "interval", aggregate)
	statement, err := sql.NewSelect(tdUsages.Name).
		WithColumns(aggregateColumn).
		WithColumns(sql.NewColumns(tdSources.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumnsWithFunctions(t.toPostgresqlAggregateFunctions(aggregate.Functions), tdUsages.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdUsages.TablePrefixedColumn("gas_source"), tdSources.TablePrefixedColumn("name"))).
		GroupBy(aggregateColumn, sql.NewColumnWithName(tdSources.TablePrefixedColumn("name"))).
		OrderAscending(aggregateColumn).
		Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usages := make([]*domain.GasUsagesRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		usages = append(usages, t.rowValuesToGasUsagesRecord(aggregate, values))
	}
	return usages, nil
}

func (t *timescaleRepository) GasUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType domain.MatchType) (*domain.GasUsageRecord, error) {
	tdUsages := t.tableDefinitions[tableGasUsages]
	tdSources := t.tableDefinitions[tableGasSources]

	filter := t.momentFilter(tdUsages.TablePrefixedColumn("time"), moment, timeMatchType)
	if sourceName != "" {
		filter.And(tdSources.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}
	if role != "" {
		filter.And(tdSources.TablePrefixedColumn("role"), sql.Equals, string(role))
	}

	selectStatement := sql.NewSelect(tdUsages.Name).
		WithColumns(sql.NewColumns(tdUsages.TablePrefixedColumnNames()[0:1]...)...).
		WithColumns(sql.NewColumns(tdSources.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumns(tdUsages.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdUsages.TablePrefixedColumn("gas_source"), tdSources.TablePrefixedColumn("name")))

	switch timeMatchType {
	case domain.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName(tdUsages.TablePrefixedColumn("time")))
		break
	case domain.EqualOrGreater:
		selectStatement.OrderAscending(sql.NewColumnWithName(tdUsages.TablePrefixedColumn("time")))
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
	return t.rowValuesToGasUsageRecord(values), nil
}

func (t *timescaleRepository) rowValuesToGasUsageRecord(values []any) *domain.GasUsageRecord {
	gasUsage := &domain.GasUsageRecord{
		Time:  values[0].(time.Time),
		Name:  values[1].(string),
		Role:  values[2].(string),
		Usage: gas.NewUsage(),
	}
	gasUsage.SetGasConsumed(values[3].(float64))
	return gasUsage
}

func (t *timescaleRepository) rowValuesToGasUsagesRecord(aggregateConfiguration *domain.AggregateConfiguration, values []any) *domain.GasUsagesRecord {
	gasUsage := &domain.GasUsagesRecord{
		StartTime: values[0].(time.Time),
		EndTime:   t.calculateEndTime(values[0].(time.Time), aggregateConfiguration),
		Name:      values[1].(string),
		Role:      values[2].(string),
		Usages:    map[domain.AggregateFunction]*gas.Usage{},
	}
	nrOfFields := 1
	for ix, aggregateFunction := range aggregateConfiguration.Functions {
		gu := gas.NewUsage()
		gu.SetGasConsumed(values[(ix*nrOfFields)+3].(float64))
		gasUsage.Usages[aggregateFunction] = gu
	}
	return gasUsage
}

type GasMeterValueChangeListener struct {
	repo *timescaleRepository
}

func (gmvcl *GasMeterValueChangeListener) HandleEvent(values *events.GasMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing gas meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.Usage() == nil || values.Usage().IsZero() {
		// No usable values in event.
		return
	}
	gmvcl.repo.registerGasSource(values.Name(), string(values.Role()))
	fields := make([]any, len(gmvcl.repo.tableDefinitions[tableGasUsages].Columns))
	fields[0] = values.EventTime()
	fields[1] = values.Name()
	fields[2] = values.Usage().GasConsumed()
	_, err = gmvcl.repo.dbPool.Exec(context.Background(), gmvcl.repo.insertQueries[tableGasUsages], fields...)
	if err != nil {
		if log.WarningEnabled() {
			log.Warningf("unable to store gas usage meter reading from '%s': %v", values.Name(), err)
		}
	}
}

func (t *timescaleRepository) registerGasSource(name string, role string) {
	cacheKey := fmt.Sprintf("gas-source-%s:%s", name, role)
	_, ok := t.energySourcesCache[cacheKey]
	if !ok {
		_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableGasSources], name, role)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("failed to register gas source '%s' with role %s: %v", name, role, err)
			}
			return
		}
		t.energySourcesCache[cacheKey] = true
	}
}

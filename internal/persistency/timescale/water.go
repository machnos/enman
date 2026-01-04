package timescale

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/repository"
	"enman/internal/domain/water"
	"enman/internal/log"
	"enman/internal/persistency/sql"
	"fmt"
	"time"
)

const (
	tableWaterSources = "water_sources"
	tableWaterUsages  = "water_usages"
)

func (t *timescaleRepository) newWaterSourcesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableWaterSources,
		Columns: []*sql.ColumnDefinition{
			{Name: "name", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "role", SqlType: "VARCHAR(25)", Nullable: false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newWaterUsagesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableWaterUsages,
		Columns: []*sql.ColumnDefinition{
			{Name: "time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "water_source", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "water_consumed", SqlType: "DOUBLE PRECISION", Nullable: true},
		},
		PrimaryKey: []string{"time", "water_source"},
		ForeignKeys: []*sql.ForeignKey{
			{SourceColumns: []string{"water_source"}, TargetTable: tableWaterSources, TargetColumns: []string{"name"}},
		},
	}
	return td
}

func (t *timescaleRepository) WaterSourceNames(from time.Time, till time.Time) ([]string, error) {
	filter := t.timeRangeFilter("time", from, till)
	statement, err := sql.NewSelect(tableWaterUsages).
		Distinct().
		WithColumns(sql.NewColumnWithName("water_source")).
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
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (t *timescaleRepository) WaterUsages(from time.Time, till time.Time, sourceName string, aggregate *repository.AggregateConfiguration) ([]*repository.WaterUsagesRecord, error) {
	tdUsages := t.tableDefinitions[tableWaterUsages]
	tdSources := t.tableDefinitions[tableWaterSources]

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
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdUsages.TablePrefixedColumn("water_source"), tdSources.TablePrefixedColumn("name"))).
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

	usages := make([]*repository.WaterUsagesRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		usages = append(usages, t.rowValuesToWaterUsagesRecord(aggregate, values))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return usages, nil
}

func (t *timescaleRepository) WaterUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType repository.MatchType) (*repository.WaterUsageRecord, error) {
	tdUsages := t.tableDefinitions[tableWaterUsages]
	tdSources := t.tableDefinitions[tableWaterSources]

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
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdUsages.TablePrefixedColumn("water_source"), tdSources.TablePrefixedColumn("name")))

	switch timeMatchType {
	case repository.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName(tdUsages.TablePrefixedColumn("time")))
		break
	case repository.EqualOrGreater:
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
	return t.rowValuesToWaterUsageRecord(values), nil
}

func (t *timescaleRepository) rowValuesToWaterUsageRecord(values []any) *repository.WaterUsageRecord {
	waterUsage := &repository.WaterUsageRecord{
		Time:  values[0].(time.Time),
		Name:  values[1].(string),
		Role:  values[2].(string),
		Usage: water.NewUsage(),
	}
	waterUsage.SetWaterConsumed(values[3].(float64))
	return waterUsage
}

func (t *timescaleRepository) rowValuesToWaterUsagesRecord(aggregateConfiguration *repository.AggregateConfiguration, values []any) *repository.WaterUsagesRecord {
	waterUsage := &repository.WaterUsagesRecord{
		StartTime: values[0].(time.Time),
		EndTime:   t.calculateEndTime(values[0].(time.Time), aggregateConfiguration),
		Name:      values[1].(string),
		Role:      values[2].(string),
		Usages:    make(map[repository.AggregateFunction]*water.Usage),
	}
	nrOfFields := 1
	for ix, aggregateFunction := range aggregateConfiguration.Functions {
		offset := ix * nrOfFields
		wu := water.NewUsage()
		wu.SetWaterConsumed(values[offset+3].(float64))
		waterUsage.Usages[aggregateFunction] = wu
	}
	return waterUsage
}

type WaterMeterValueChangeListener struct {
	repo *timescaleRepository
}

func (wmvcl *WaterMeterValueChangeListener) HandleEvent(values *events.WaterMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing water meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.Usage() == nil || values.Usage().IsZero() {
		// No usable values in event.
		return
	}
	wmvcl.repo.registerWaterSource(values.Name(), string(values.Role()))
	fields := make([]any, len(wmvcl.repo.tableDefinitions[tableWaterUsages].Columns))
	fields[0] = values.EventTime()
	fields[1] = values.Name()
	fields[2] = values.Usage().WaterConsumed()
	_, err = wmvcl.repo.dbPool.Exec(context.Background(), wmvcl.repo.insertQueries[tableWaterUsages], fields...)
	if err != nil {
		if log.WarningEnabled() {
			log.Warningf("unable to store water usage meter reading from '%s': %v", values.Name(), err)
		}
	}
}

func (t *timescaleRepository) registerWaterSource(name string, role string) {
	cacheKey := fmt.Sprintf("water-source-%s:%s", name, role)
	_, ok := t.energySourcesCache[cacheKey]
	if !ok {
		_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableWaterSources], name, role)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("failed to register water source '%s' with role %s: %v", name, role, err)
			}
			return
		}
		t.energySourcesCache[cacheKey] = true
	}
}

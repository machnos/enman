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
	tablePrefixElectricity  = "electricity_"
	tableElectricitySources = tablePrefixElectricity + "sources"
	tableElectricityStates  = tablePrefixElectricity + "states"
	tableElectricityUsages  = tablePrefixElectricity + "usages"
	tableElectricityCosts   = tablePrefixElectricity + "costs"
)

func (t *timescaleRepository) newElectricitySourcesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableElectricitySources,
		Columns: []*sql.ColumnDefinition{
			{"name", "VARCHAR(50)", false},
			{"role", "VARCHAR(25)", false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newElectricityStatesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableElectricityStates,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"electricity_source", "VARCHAR(50)", false},
			{"total_current", "DOUBLE PRECISION", true},
			{"total_power", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "electricity_source"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"electricity_source"}, tableElectricitySources, []string{"name"}},
		},
	}
	td.Columns = append(td.Columns, t.columnDefinitionPerPhase("current", "DOUBLE PRECISION", true)...)
	td.Columns = append(td.Columns, t.columnDefinitionPerPhase("power", "DOUBLE PRECISION", true)...)
	td.Columns = append(td.Columns, t.columnDefinitionPerPhase("voltage", "DOUBLE PRECISION", true)...)
	return td
}

func (t *timescaleRepository) newElectricityUsagesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableElectricityUsages,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"electricity_source", "VARCHAR(50)", false},
			{"total_energy_consumed", "DOUBLE PRECISION", true},
			{"total_energy_provided", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "electricity_source"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"electricity_source"}, tableElectricitySources, []string{"name"}},
		},
	}
	td.Columns = append(td.Columns, t.columnDefinitionPerPhase("energy_consumed", "DOUBLE PRECISION", true)...)
	td.Columns = append(td.Columns, t.columnDefinitionPerPhase("energy_provided", "DOUBLE PRECISION", true)...)
	return td
}

func (t *timescaleRepository) newElectricityCostsDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableElectricityCosts,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"electricity_provider", "VARCHAR(50)", false},
			{"total_energy_consumed", "DOUBLE PRECISION", true},
			{"consumption_price_per_kwh", "DOUBLE PRECISION", true},
			{"consumption_costs", "DOUBLE PRECISION", true},
			{"total_energy_provided", "DOUBLE PRECISION", true},
			{"feedback_price_per_kwh", "DOUBLE PRECISION", true},
			{"feedback_costs", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "electricity_provider"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"electricity_provider"}, tableEnergyPriceProviders, []string{"name"}},
		},
	}
}

func (t *timescaleRepository) ElectricitySourceNames(from time.Time, till time.Time) ([]string, error) {
	statement, err := sql.NewSelect(tableElectricityStates).
		WithColumns(sql.NewDistinctColumn("electricity_source")).
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

func (t *timescaleRepository) ElectricityUsages(
	from time.Time,
	till time.Time,
	sourceName string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.ElectricityUsageRecord, error) {
	tdUsages := t.tableDefinitions[tableElectricityUsages]
	tdSources := t.tableDefinitions[tableElectricitySources]

	filter := t.timeRangeFilter(tdUsages.TablePrefixedColumn("time"), from, till)
	if sourceName != "" {
		filter.And(tdSources.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}
	aggregateColumn := t.toAggregateWindowColumn(tdUsages.TablePrefixedColumn("time"), "interval", aggregate)
	statement, err := sql.NewSelect(tableElectricityUsages).
		WithColumns(aggregateColumn).
		WithColumns(sql.NewColumns(t.tableDefinitions[tableElectricitySources].TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumnsWithFunction(t.toPostgresqlAggregateFunction(aggregate.Function), t.tableDefinitions[tableElectricityUsages].TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tableElectricitySources, sql.Inner, tdUsages.TablePrefixedColumn("electricity_source"), tdSources.TablePrefixedColumn("name"))).
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

	usages := make([]*domain.ElectricityUsageRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		usages = append(usages, t.rowValuesToElectricityUsageRecord(values))
	}
	return usages, nil
}

func (t *timescaleRepository) ElectricityUsageAtTime(moment time.Time, sourceName string, role domain.EnergySourceRole, timeMatchType domain.MatchType) (*domain.ElectricityUsageRecord, error) {
	tdUsages := t.tableDefinitions[tableElectricityUsages]
	tdSources := t.tableDefinitions[tableElectricitySources]

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
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdUsages.TablePrefixedColumn("electricity_source"), tdSources.TablePrefixedColumn("name")))

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
	return t.rowValuesToElectricityUsageRecord(values), nil
}

func (t *timescaleRepository) ElectricityStates(from time.Time, till time.Time, sourceName string, aggregate *domain.AggregateConfiguration) ([]*domain.ElectricityStateRecord, error) {
	tdStates := t.tableDefinitions[tableElectricityStates]
	tdSources := t.tableDefinitions[tableElectricitySources]

	filter := t.timeRangeFilter(tdStates.TablePrefixedColumn("time"), from, till)
	if sourceName != "" {
		filter.And(tdSources.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}

	aggregateColumn := t.toAggregateWindowColumn(tdStates.TablePrefixedColumn("time"), "interval", aggregate)
	statement, err := sql.NewSelect(tdStates.Name).
		WithColumns(aggregateColumn).
		WithColumns(sql.NewColumns(tdSources.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumnsWithFunction(t.toPostgresqlAggregateFunction(aggregate.Function), tdStates.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdSources.Name, sql.Inner, tdStates.TablePrefixedColumn("electricity_source"), tdSources.TablePrefixedColumn("name"))).
		GroupBy(aggregateColumn, sql.NewColumnWithName(tdSources.TablePrefixedColumn("name"))).
		OrderAscending(aggregateColumn).Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]*domain.ElectricityStateRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		states = append(states, t.rowValuesToElectricityStateRecord(values))
	}
	return states, nil
}

func (t *timescaleRepository) ElectricityCosts(from time.Time, till time.Time, providerName string, aggregate *domain.AggregateConfiguration) ([]*domain.ElectricityCostsRecord, error) {
	tdCosts := t.tableDefinitions[tableElectricityCosts]
	tdProviders := t.tableDefinitions[tableEnergyPriceProviders]

	filter := t.timeRangeFilter(tdCosts.TablePrefixedColumn("time"), from, till)
	if providerName != "" {
		filter.And(tdProviders.TablePrefixedColumn("name"), sql.Equals, providerName)
	}
	aggregateColumn := t.toAggregateWindowColumn(tdCosts.TablePrefixedColumn("time"), "interval", aggregate)
	statement, err := sql.NewSelect(tdCosts.Name).
		WithColumns(aggregateColumn).
		WithColumns(sql.NewColumns(tdProviders.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumnsWithFunction(t.toPostgresqlAggregateFunction(aggregate.Function), tdCosts.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdProviders.Name, sql.Inner, tdCosts.TablePrefixedColumn("electricity_provider"), tdProviders.TablePrefixedColumn("name"))).
		GroupBy(aggregateColumn, sql.NewColumnWithName(tdProviders.TablePrefixedColumn("name"))).
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

	usages := make([]*domain.ElectricityCostsRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		usages = append(usages, t.rowValuesToElectricityCostsRecord(values))
	}
	return usages, nil
}

func (t *timescaleRepository) rowValuesToElectricityUsageRecord(values []any) *domain.ElectricityUsageRecord {
	electricityUsage := &domain.ElectricityUsageRecord{
		Time:             values[0].(time.Time),
		Name:             values[1].(string),
		Role:             values[2].(string),
		ElectricityUsage: domain.NewElectricityUsage(),
	}
	electricityUsage.ElectricityUsage.SetTotalEnergyConsumed(values[3].(float64))
	electricityUsage.ElectricityUsage.SetTotalEnergyProvided(values[4].(float64))
	for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
		electricityUsage.SetEnergyConsumed(lineIx, values[5+lineIx].(float64))
		electricityUsage.SetEnergyProvided(lineIx, values[5+domain.MaxPhases+lineIx].(float64))
	}
	return electricityUsage
}

func (t *timescaleRepository) rowValuesToElectricityStateRecord(values []any) *domain.ElectricityStateRecord {
	electricityUsage := &domain.ElectricityStateRecord{
		Time:             values[0].(time.Time),
		Name:             values[1].(string),
		Role:             values[2].(string),
		ElectricityState: domain.NewElectricityState(),
	}
	for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
		electricityUsage.SetCurrent(lineIx, float32(values[5+lineIx].(float64)))
		electricityUsage.SetPower(lineIx, float32(values[5+domain.MaxPhases+lineIx].(float64)))
		electricityUsage.SetVoltage(lineIx, float32(values[5+domain.MaxPhases+domain.MaxPhases+lineIx].(float64)))
	}
	return electricityUsage
}

func (t *timescaleRepository) rowValuesToElectricityCostsRecord(values []any) *domain.ElectricityCostsRecord {
	return &domain.ElectricityCostsRecord{
		Time:                   values[0].(time.Time),
		Name:                   values[1].(string),
		ConsumptionEnergy:      float32(values[2].(float64)),
		ConsumptionPricePerKwh: float32(values[3].(float64)),
		ConsumptionCosts:       float32(values[4].(float64)),
		FeedbackEnergy:         float32(values[5].(float64)),
		FeedbackPricePerKwh:    float32(values[6].(float64)),
		FeedbackCosts:          float32(values[7].(float64)),
	}
}

type ElectricityMeterValueChangeListener struct {
	repo *timescaleRepository
}

func (emvcl *ElectricityMeterValueChangeListener) HandleEvent(values *domain.ElectricityMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("not storing electricity meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.ElectricityState() == nil && values.ElectricityUsage() == nil {
		// No usable values in event.
		return
	}
	if values.ElectricityState() != nil {
		emvcl.repo.registerElectricitySource(values.Name(), string(values.Role()))
		fields := make([]any, len(emvcl.repo.tableDefinitions[tableElectricityStates].Columns))
		fields[0] = values.EventTime()
		fields[1] = values.Name()
		fields[2] = values.ElectricityState().TotalCurrent()
		fields[3] = values.ElectricityState().TotalPower()

		for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
			fields[4+lineIx] = values.ElectricityState().Current(lineIx)
			fields[4+domain.MaxPhases+lineIx] = values.ElectricityState().Power(lineIx)
			fields[4+domain.MaxPhases+domain.MaxPhases+lineIx] = values.ElectricityState().Voltage(lineIx)
		}
		_, err = emvcl.repo.dbPool.Exec(context.Background(), emvcl.repo.insertQueries[tableElectricityStates], fields...)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("unable to store electricity state meter reading from '%s': %v", values.Name(), err)
			}
		}
	}
	if values.ElectricityUsage() != nil {
		emvcl.repo.registerElectricitySource(values.Name(), string(values.Role()))
		fields := make([]any, len(emvcl.repo.tableDefinitions[tableElectricityUsages].Columns))
		fields[0] = values.EventTime()
		fields[1] = values.Name()
		fields[2] = values.ElectricityUsage().TotalEnergyConsumed()
		fields[3] = values.ElectricityUsage().TotalEnergyProvided()

		for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
			fields[4+lineIx] = values.ElectricityUsage().EnergyConsumed(lineIx)
			fields[4+domain.MaxPhases+lineIx] = values.ElectricityUsage().EnergyProvided(lineIx)
		}
		_, err = emvcl.repo.dbPool.Exec(context.Background(), emvcl.repo.insertQueries[tableElectricityUsages], fields...)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("unable to store electricity usage meter reading from '%s': %v", values.Name(), err)
			}
		}
	}
}

type ElectricityCostsValueChangeListener struct {
	repo *timescaleRepository
}

func (ecvcl *ElectricityCostsValueChangeListener) HandleEvent(values *domain.ElectricityCostsValues) {
	fields := make([]any, len(ecvcl.repo.tableDefinitions[tableElectricityCosts].Columns))
	fields[0] = values.EventTime()
	fields[1] = values.EnergyProviderName()
	fields[2] = values.ConsumptionEnergy()
	fields[3] = values.ConsumptionPricePerKwh()
	fields[4] = values.ConsumptionCosts()
	fields[5] = values.FeedbackEnergy()
	fields[6] = values.FeedbackPricePerKwh()
	fields[7] = values.FeedbackCosts()
	_, err := ecvcl.repo.dbPool.Exec(context.Background(), ecvcl.repo.insertQueries[tableElectricityCosts], fields...)
	if err != nil {
		if log.WarningEnabled() {
			log.Warningf("unable to store electricity costs from '%s': %v", values.EnergyProviderName(), err)
		}
	}
}

func (t *timescaleRepository) registerElectricitySource(name string, role string) {
	cacheKey := fmt.Sprintf("electricity-source-%s:%s", name, role)
	_, ok := t.energySourcesCache[cacheKey]
	if !ok {
		_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableElectricitySources], name, role)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("failed to register electricity source '%s' with role %s: %v", name, role, err)
			}
			return
		}
		t.energySourcesCache[cacheKey] = true
	}
}

func (t *timescaleRepository) columnDefinitionPerPhase(columnName string, sqlType string, nullable bool) []*sql.ColumnDefinition {
	result := make([]*sql.ColumnDefinition, 0)
	for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
		result = append(result, &sql.ColumnDefinition{
			Name:     fmt.Sprintf("%s_l%d", columnName, lineIx+1),
			SqlType:  sqlType,
			Nullable: nullable,
		})
	}
	return result
}

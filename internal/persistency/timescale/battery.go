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
	tableBatteries     = "batteries"
	tableBatteryStates = "battery_states"
)

func (t *timescaleRepository) newBatteriesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableBatteries,
		Columns: []*sql.ColumnDefinition{
			{"name", "VARCHAR(50)", false},
			{"role", "VARCHAR(25)", false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newBatteryStatesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableBatteryStates,
		Columns: []*sql.ColumnDefinition{
			{"time", "TIMESTAMPTZ", false},
			{"battery", "VARCHAR(50)", false},
			{"current", "DOUBLE PRECISION", true},
			{"power", "DOUBLE PRECISION", true},
			{"voltage", "DOUBLE PRECISION", true},
			{"soc", "DOUBLE PRECISION", true},
			{"soh", "DOUBLE PRECISION", true},
		},
		PrimaryKey: []string{"time", "battery"},
		ForeignKeys: []*sql.ForeignKey{
			{[]string{"battery"}, tableBatteries, []string{"name"}},
		},
	}
	return td
}

func (t *timescaleRepository) BatterySourceNames(from time.Time, till time.Time) ([]string, error) {
	statement, err := sql.NewSelect(tableBatteryStates).
		WithColumns(sql.NewDistinctColumn("battery")).
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

func (t *timescaleRepository) BatteryStates(from time.Time, till time.Time, sourceName string, aggregate *domain.AggregateConfiguration) ([]*domain.BatteryStateRecord, error) {
	tdStates := t.tableDefinitions[tableBatteryStates]
	tdBatteries := t.tableDefinitions[tableBatteries]

	filter := t.timeRangeFilter(tdStates.TablePrefixedColumn("time"), from, till)
	if sourceName != "" {
		filter.And(tdBatteries.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}

	aggregateColumn := t.toAggregateWindowColumn(tdStates.TablePrefixedColumn("time"), "interval", aggregate)
	statement, err := sql.NewSelect(tdStates.Name).
		WithColumns(aggregateColumn).
		WithColumns(sql.NewColumns(tdBatteries.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumnsWithFunction(t.toPostgresqlAggregateFunction(aggregate.Function), tdStates.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdBatteries.Name, sql.Inner, tdStates.TablePrefixedColumn("battery"), tdBatteries.TablePrefixedColumn("name"))).
		GroupBy(aggregateColumn, sql.NewColumnWithName(tdBatteries.TablePrefixedColumn("name"))).
		OrderAscending(aggregateColumn).Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]*domain.BatteryStateRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		states = append(states, t.rowValuesToBatteryStateRecord(values))
	}
	return states, nil
}

func (t *timescaleRepository) BatteryStateAtTime(moment time.Time, sourceName string, role domain.EnergySourceRole, timeMatchType domain.MatchType) (*domain.BatteryStateRecord, error) {
	tdStates := t.tableDefinitions[tableElectricityStates]
	tdBatteries := t.tableDefinitions[tableBatteries]

	filter := t.momentFilter(tdStates.TablePrefixedColumn("time"), moment, timeMatchType)
	if sourceName != "" {
		filter.And(tdBatteries.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}
	if role != "" {
		filter.And(tdBatteries.TablePrefixedColumn("role"), sql.Equals, string(role))
	}

	selectStatement := sql.NewSelect(tdStates.Name).
		WithColumns(sql.NewColumns(tdStates.TablePrefixedColumnNames()[0:1]...)...).
		WithColumns(sql.NewColumns(tdBatteries.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumns(tdStates.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdBatteries.Name, sql.Inner, tdStates.TablePrefixedColumn("battery"), tdBatteries.TablePrefixedColumn("name")))

	switch timeMatchType {
	case domain.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName(tdStates.TablePrefixedColumn("time")))
		break
	case domain.EqualOrGreater:
		selectStatement.OrderAscending(sql.NewColumnWithName(tdStates.TablePrefixedColumn("time")))
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
	return t.rowValuesToBatteryStateRecord(values), nil
}

func (t *timescaleRepository) rowValuesToBatteryStateRecord(values []any) *domain.BatteryStateRecord {
	batteryState := &domain.BatteryStateRecord{
		Time:         values[0].(time.Time),
		Name:         values[1].(string),
		Role:         values[2].(string),
		BatteryState: domain.NewBatteryState(),
	}
	batteryState.SetCurrent(float32(values[3].(float64)))
	batteryState.SetPower(float32(values[4].(float64)))
	batteryState.SetVoltage(float32(values[5].(float64)))
	batteryState.SetSoC(float32(values[6].(float64)))
	batteryState.SetSoH(float32(values[7].(float64)))
	return batteryState
}

type BatteryMeterValueChangeListener struct {
	repo *timescaleRepository
}

func (bmvcl *BatteryMeterValueChangeListener) HandleEvent(values *domain.BatteryMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing battery meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.BatteryState() == nil || values.BatteryState().IsZero() {
		// No usable values in event.
		return
	}
	bmvcl.repo.registerBattery(values.Name(), string(values.Role()))
	fields := make([]any, len(bmvcl.repo.tableDefinitions[tableBatteryStates].Columns))
	fields[0] = values.EventTime()
	fields[1] = values.Name()
	fields[2] = values.BatteryState().Current()
	fields[3] = values.BatteryState().Power()
	fields[4] = values.BatteryState().Voltage()
	fields[5] = values.BatteryState().SoC()
	fields[6] = values.BatteryState().SoH()
	_, err = bmvcl.repo.dbPool.Exec(context.Background(), bmvcl.repo.insertQueries[tableBatteryStates], fields...)
	if err != nil {
		if log.WarningEnabled() {
			log.Warningf("unable to store battery state meter reading from '%s': %v", values.Name(), err)
		}
	}
}

func (t *timescaleRepository) registerBattery(name string, role string) {
	cacheKey := fmt.Sprintf("battery-source-%s:%s", name, role)
	_, ok := t.energySourcesCache[cacheKey]
	if !ok {
		_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableBatteries], name, role)
		if err != nil {
			if log.WarningEnabled() {
				log.Warningf("failed to register battery '%s' with role %s: %v", name, role, err)
			}
			return
		}
		t.energySourcesCache[cacheKey] = true
	}
}

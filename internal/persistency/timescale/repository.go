package timescale

import (
	"context"
	"enman/internal/domain"
	"enman/internal/domain/events"
	"enman/internal/log"
	"enman/internal/persistency/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type timescaleRepository struct {
	domain.Repository
	dbPool             *pgxpool.Pool
	tableDefinitions   map[string]*sql.TableDefinition
	energySourcesCache map[string]bool
	insertQueries      map[string]string
}

func NewTimescaleRepository(connectionString string) (domain.Repository, error) {
	dbPool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return nil, err
	}
	return &timescaleRepository{
		dbPool:             dbPool,
		tableDefinitions:   make(map[string]*sql.TableDefinition),
		energySourcesCache: make(map[string]bool),
		insertQueries:      make(map[string]string),
	}, nil
}

func (t *timescaleRepository) Initialize() error {
	err := t.initializeTable(t.newElectricitySourcesDefinition())
	if err != nil {
		return err
	}
	err = t.initializeTable(t.newEnergyProvidersDefinition())
	if err != nil {
		return err
	}
	err = t.initializeTable(t.newEnergyPricesDefinition())
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newElectricityStatesDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newElectricityUsagesDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newElectricityCostsDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeTable(t.newGasSourcesDefinition())
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newGasUsagesDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newGasCostsDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeTable(t.newWaterSourcesDefinition())
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newWaterUsagesDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeTable(t.newBatteriesDefinition())
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newBatteryStatesDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}
	err = t.initializeHyperTable(t.newForecastsDefinition(), "time", uint32((time.Hour * 24 * 90).Hours()))
	if err != nil {
		return err
	}

	events.ElectricityMeterReadings.Register(&ElectricityMeterValueChangeListener{repo: t}, nil)
	events.ElectricityCosts.Register(&ElectricityCostsValueChangeListener{repo: t}, nil)
	events.GasMeterReadings.Register(&GasMeterValueChangeListener{repo: t}, nil)
	events.GasCosts.Register(&GasCostsValueChangeListener{repo: t}, nil)
	events.WaterMeterReadings.Register(&WaterMeterValueChangeListener{repo: t}, nil)
	events.BatteryMeterReadings.Register(&BatteryMeterValueChangeListener{repo: t}, nil)
	events.Forecasts.Register(&ForecastValueChangeListener{repo: t}, nil)

	return nil
}

func (t *timescaleRepository) initializeTable(td *sql.TableDefinition) error {
	t.tableDefinitions[td.Name] = td
	t.insertQueries[td.Name] = td.UpsertStatement(sql.Timescale)
	exists, err := t.checkTableExists(td.Name)
	if err != nil {
		return err
	}
	if !exists {
		_, err = t.dbPool.Exec(context.Background(), td.CreateStatement())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *timescaleRepository) initializeHyperTable(td *sql.TableDefinition, timeColumn string, retentionHours uint32) error {
	err := t.initializeTable(td)
	if err != nil {
		return err
	}
	exists, err := t.checkHyperTableExists(td.Name)
	if err != nil {
		return err
	}
	if !exists {
		_, err = t.dbPool.Exec(context.Background(), fmt.Sprintf("SELECT create_hypertable('%s', by_range('%s'));", td.Name, timeColumn))
		if err != nil {
			return err
		}
	}
	exists, err = t.checkRetentionPolicyExists(td.Name)
	if err != nil {
		return err
	}
	if !exists {
		_, err = t.dbPool.Exec(context.Background(), fmt.Sprintf("SELECT add_retention_policy('%s', INTERVAL '%d hours');", td.Name, retentionHours))
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *timescaleRepository) Close() {
	t.dbPool.Close()
}

func (t *timescaleRepository) rowCount(table string, filterFunction *sql.FilterFunction) (int, error) {
	selectStatement := sql.NewSelect(table).WithColumns(sql.NewCountColumn("*"))
	if filterFunction != nil {
		selectStatement.WithFilter(filterFunction)
	}
	query, err := selectStatement.Build()
	if err != nil {
		return 0, err
	}
	rows, err := t.dbPool.Query(context.Background(), query.Query, query.Args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int
	if !rows.Next() {
		return 0, nil
	}
	err = rows.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (t *timescaleRepository) checkTableExists(table string) (bool, error) {
	filterFunction := sql.NewFilterFunction("table_name", sql.Equals, table)
	count, err := t.rowCount("information_schema.tables", filterFunction)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (t *timescaleRepository) checkHyperTableExists(table string) (bool, error) {
	filterFunction := sql.NewFilterFunction("hypertable_name", sql.Equals, table)
	count, err := t.rowCount("timescaledb_information.hypertables", filterFunction)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (t *timescaleRepository) checkRetentionPolicyExists(table string) (bool, error) {
	filterFunction := sql.NewFilterFunction("hypertable_name", sql.Equals, table).And("proc_name", sql.Equals, "policy_retention")
	count, err := t.rowCount("timescaledb_information.jobs", filterFunction)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (t *timescaleRepository) timeRangeFilter(column string, from time.Time, till time.Time) *sql.FilterFunction {
	filter := sql.NewFilterFunction(column, sql.GreaterThanOrEqual, from)
	if !till.IsZero() {
		filter.And(column, sql.LessThanOrEquals, till)
	}
	return filter
}

func (t *timescaleRepository) momentFilter(column string, moment time.Time, matchType domain.MatchType) *sql.FilterFunction {
	var operator sql.ComparisonOperator
	switch matchType {
	case domain.LessOrEqual:
		operator = sql.LessThanOrEquals
		break
	case domain.EqualOrGreater:
		operator = sql.GreaterThanOrEqual
		break
	case domain.Equal:
		operator = sql.Equals
		break
	}
	return sql.NewFilterFunction(column, operator, moment)
}

func (t *timescaleRepository) toAggregateWindowColumn(column string, as string, ac *domain.AggregateConfiguration) *sql.Column {
	return sql.NewColumn(column, as, func(col string) string {
		return fmt.Sprintf("time_bucket('%s', %s)", t.toPostgresqlInterval(ac.WindowUnit, ac.WindowAmount), col)
	})
}

func (t *timescaleRepository) toPostgresqlInterval(unit domain.WindowUnit, amount uint64) string {
	switch unit {
	case domain.WindowUnitNanosecond:
		return "nanosecond unsupported"
	default:
		return fmt.Sprintf("%d %s", amount, unit.String())
	}
}

func (t *timescaleRepository) toPostgresqlAggregateFunctions(functions []domain.AggregateFunction) []func(string) string {
	result := make([]func(string) string, 0)
	for _, function := range functions {
		switch function {
		case domain.AggregateFunctionCount:
			result = append(result, func(col string) string {
				return fmt.Sprintf("COUNT(%s)", col)
			})
			break
		case domain.AggregateFunctionMax:
			result = append(result, func(col string) string {
				return fmt.Sprintf("MAX(%s)", col)
			})
			break
		case domain.AggregateFunctionMean:
			result = append(result, func(col string) string {
				return fmt.Sprintf("AVG(%s)", col)
			})
			break
		case domain.AggregateFunctionMedian:
			result = append(result, func(col string) string {
				return fmt.Sprintf("MEDIAN(%s)", col)
			})
			break
		case domain.AggregateFunctionMin:
			result = append(result, func(col string) string {
				return fmt.Sprintf("MIN(%s)", col)
			})
			break
		case domain.AggregateFunctionSum:
			result = append(result, func(col string) string {
				return fmt.Sprintf("SUM(%s)", col)
			})
			break
		default:
			log.Warningf("unknown AggregateFunction type: %s", function)
			result = append(result, func(col string) string {
				return col
			})
			break
		}
	}
	return result
}

func (t *timescaleRepository) calculateEndTime(startTime time.Time, ac *domain.AggregateConfiguration) time.Time {
	switch ac.WindowUnit {
	case domain.WindowUnitNanosecond:
		return startTime.Add(time.Nanosecond * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitMicrosecond:
		return startTime.Add(time.Microsecond * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitMillisecond:
		return startTime.Add(time.Millisecond * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitSecond:
		return startTime.Add(time.Second * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitMinute:
		return startTime.Add(time.Minute * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitHour:
		return startTime.Add(time.Hour * time.Duration(ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitDay:
		return startTime.AddDate(0, 0, int(1*ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitWeek:
		return startTime.AddDate(0, 0, int(7*ac.WindowAmount)).Add(time.Nanosecond * -1)
	case domain.WindowUnitMonth:
		return startTime.AddDate(0, int(1*ac.WindowAmount), 0).Add(time.Nanosecond * -1)
	case domain.WindowUnitYear:
		return startTime.AddDate(int(1*ac.WindowAmount), 0, 0).Add(time.Nanosecond * -1)
	}
	return startTime
}

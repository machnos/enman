package timescale

import (
	"context"
	"enman/internal/domain"
	"enman/internal/persistency/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
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

	domain.ElectricityMeterReadings.Register(&ElectricityMeterValueChangeListener{repo: t}, nil)
	domain.ElectricityCosts.Register(&ElectricityCostsValueChangeListener{repo: t}, nil)
	domain.GasMeterReadings.Register(&GasMeterValueChangeListener{repo: t}, nil)
	domain.WaterMeterReadings.Register(&WaterMeterValueChangeListener{repo: t}, nil)
	domain.BatteryMeterReadings.Register(&BatteryMeterValueChangeListener{repo: t}, nil)

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
	case domain.WindowUnitMicrosecond:
		return fmt.Sprintf("%d microsecond", amount)
	case domain.WindowUnitMillisecond:
		return fmt.Sprintf("%d millisecond", amount)
	case domain.WindowUnitSecond:
		return fmt.Sprintf("%d second", amount)
	case domain.WindowUnitMinute:
		return fmt.Sprintf("%d minute", amount)
	case domain.WindowUnitHour:
		return fmt.Sprintf("%d hour", amount)
	case domain.WindowUnitDay:
		return fmt.Sprintf("%d day", amount)
	case domain.WindowUnitWeek:
		return fmt.Sprintf("%d week", amount)
	case domain.WindowUnitMonth:
		return fmt.Sprintf("%d month", amount)
	case domain.WindowUnitYear:
		return fmt.Sprintf("%d year", amount)
	}
	return ""
}

func (t *timescaleRepository) toPostgresqlAggregateFunction(function domain.AggregateFunction) func(string) string {
	switch function.(type) {
	case domain.Count:
		return func(col string) string {
			return fmt.Sprintf("COUNT(%s)", col)
		}
	case domain.Max:
		return func(col string) string {
			return fmt.Sprintf("MAX(%s)", col)
		}
	case domain.Mean:
		return func(col string) string {
			return fmt.Sprintf("AVG(%s)", col)
		}
	case domain.Median:
		return func(col string) string {
			return fmt.Sprintf("MEDIAN(%s)", col)
		}
	case domain.Min:
		return func(col string) string {
			return fmt.Sprintf("MIN(%s)", col)
		}
	case domain.Sum:
		return func(col string) string {
			return fmt.Sprintf("SUM(%s)", col)
		}
	default:
		return func(col string) string {
			return col
		}
	}
}

package timescale

import (
	"context"
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"enman/internal/persistency/sql"
	"fmt"
	"time"
)

const (
	tableBatteries            = "batteries"
	tableBatteryStates        = "battery_states"
	tableBatteryScheduleSlots = "battery_schedule_slots"
)

func (t *timescaleRepository) newBatteriesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableBatteries,
		Columns: []*sql.ColumnDefinition{
			{Name: "name", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "role", SqlType: "VARCHAR(25)", Nullable: false},
		},
		PrimaryKey: []string{"name"},
	}
}

func (t *timescaleRepository) newBatteryStatesDefinition() *sql.TableDefinition {
	td := &sql.TableDefinition{
		Name: tableBatteryStates,
		Columns: []*sql.ColumnDefinition{
			{Name: "time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "battery", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "current", SqlType: "DOUBLE PRECISION", Nullable: true},
			{Name: "power", SqlType: "DOUBLE PRECISION", Nullable: true},
			{Name: "voltage", SqlType: "DOUBLE PRECISION", Nullable: true},
			{Name: "soc", SqlType: "DOUBLE PRECISION", Nullable: true},
			{Name: "soh", SqlType: "DOUBLE PRECISION", Nullable: true},
		},
		PrimaryKey: []string{"time", "battery"},
		ForeignKeys: []*sql.ForeignKey{
			{SourceColumns: []string{"battery"}, TargetTable: tableBatteries, TargetColumns: []string{"name"}},
		},
	}
	return td
}

func (t *timescaleRepository) BatterySourceNames(from time.Time, till time.Time) ([]string, error) {
	statement, err := sql.NewSelect(tableBatteryStates).
		Distinct().
		WithColumns(sql.NewColumnWithName("battery")).
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
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (t *timescaleRepository) BatteryStates(from time.Time, till time.Time, sourceName string, aggregate *repository.AggregateConfiguration) ([]*repository.BatteryStatesRecord, error) {
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
		WithColumns(sql.NewColumnsWithFunctions(t.toPostgresqlAggregateFunctions(aggregate.Functions), tdStates.TablePrefixedColumnNames()[2:]...)...).
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

	states := make([]*repository.BatteryStatesRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		states = append(states, t.rowValuesToBatteryStatesRecord(aggregate, values))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}

func (t *timescaleRepository) BatteryStateAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType repository.MatchType) (*repository.BatteryStateRecord, error) {
	tdStates := t.tableDefinitions[tableBatteryStates]
	tdBatteries := t.tableDefinitions[tableBatteries]

	filter := t.momentFilter(tdStates.TablePrefixedColumn("time"), moment, timeMatchType)
	if sourceName != "" {
		filter.And(tdBatteries.TablePrefixedColumn("name"), sql.Equals, sourceName)
	}
	if role != constants.EnergySourceRoleUndefined {
		filter.And(tdBatteries.TablePrefixedColumn("role"), sql.Equals, string(role))
	}

	selectStatement := sql.NewSelect(tdStates.Name).
		WithColumns(sql.NewColumns(tdStates.TablePrefixedColumnNames()[0:1]...)...).
		WithColumns(sql.NewColumns(tdBatteries.TablePrefixedColumnNames()...)...).
		WithColumns(sql.NewColumns(tdStates.TablePrefixedColumnNames()[2:]...)...).
		WithFilter(filter).
		WithJoin(sql.NewJoin(tdBatteries.Name, sql.Inner, tdStates.TablePrefixedColumn("battery"), tdBatteries.TablePrefixedColumn("name")))

	switch timeMatchType {
	case repository.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName(tdStates.TablePrefixedColumn("time")))
		break
	case repository.EqualOrGreater:
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

func (t *timescaleRepository) rowValuesToBatteryStateRecord(values []any) *repository.BatteryStateRecord {
	batteryState := &repository.BatteryStateRecord{
		Time:  values[0].(time.Time),
		Name:  values[1].(string),
		Role:  values[2].(string),
		State: battery.NewState(),
	}
	batteryState.SetCurrent(float32(values[3].(float64)))
	batteryState.SetPower(float32(values[4].(float64)))
	batteryState.SetVoltage(float32(values[5].(float64)))
	batteryState.SetSoC(float32(values[6].(float64)))
	batteryState.SetSoH(float32(values[7].(float64)))
	return batteryState
}

func (t *timescaleRepository) rowValuesToBatteryStatesRecord(aggregateConfiguration *repository.AggregateConfiguration, values []any) *repository.BatteryStatesRecord {
	batteryStates := &repository.BatteryStatesRecord{
		StartTime: values[0].(time.Time),
		EndTime:   t.calculateEndTime(values[0].(time.Time), aggregateConfiguration),
		Name:      values[1].(string),
		Role:      values[2].(string),
		States:    make(map[repository.AggregateFunction]*battery.State),
	}
	nrOfFields := 5
	for ix, aggregateFunction := range aggregateConfiguration.Functions {
		offset := ix * nrOfFields
		batteryState := battery.NewState()
		batteryState.SetCurrent(float32(values[offset+3].(float64)))
		batteryState.SetPower(float32(values[offset+4].(float64)))
		batteryState.SetVoltage(float32(values[offset+5].(float64)))
		batteryState.SetSoC(float32(values[offset+6].(float64)))
		batteryState.SetSoH(float32(values[offset+7].(float64)))
		batteryStates.States[aggregateFunction] = batteryState
	}
	return batteryStates
}

type BatteryMeterValueChangeListener struct {
	repo *timescaleRepository
}

func (bmvcl *BatteryMeterValueChangeListener) HandleEvent(values *events.BatteryMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing battery meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.State() == nil || values.State().IsZero() {
		// No usable values in event.
		return
	}
	bmvcl.repo.registerBattery(values.Name(), string(values.Role()))
	fields := make([]any, len(bmvcl.repo.tableDefinitions[tableBatteryStates].Columns))
	fields[0] = values.EventTime()
	fields[1] = values.Name()
	fields[2] = values.State().Current()
	fields[3] = values.State().Power()
	fields[4] = values.State().Voltage()
	fields[5] = values.State().SoC()
	fields[6] = values.State().SoH()
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

// Battery Schedule Slot methods

func (t *timescaleRepository) newBatteryScheduleSlotsDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableBatteryScheduleSlots,
		Columns: []*sql.ColumnDefinition{
			{Name: "start_time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "end_time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "charge_power", SqlType: "DOUBLE PRECISION", Nullable: false},
			{Name: "predicted_soc", SqlType: "DOUBLE PRECISION", Nullable: false},
			{Name: "price_per_kwh", SqlType: "DOUBLE PRECISION", Nullable: false},
			{Name: "charging_source", SqlType: "VARCHAR(10)", Nullable: false},
		},
		PrimaryKey: []string{"start_time"},
	}
}

// rowValuesToScheduleSlotRecord converts row values to a BatteryScheduleSlotRecord
func (t *timescaleRepository) rowValuesToScheduleSlotRecord(values []any) *repository.BatteryScheduleSlotRecord {
	return &repository.BatteryScheduleSlotRecord{
		StartTime:      values[0].(time.Time),
		EndTime:        values[1].(time.Time),
		ChargePower:    float32(values[2].(float64)),
		PredictedSoC:   float32(values[3].(float64)),
		PricePerKwh:    float32(values[4].(float64)),
		ChargingSource: values[5].(string),
	}
}

// storeScheduleSlot persists a battery schedule slot to the database (called by event listener)
func (t *timescaleRepository) storeScheduleSlot(slot *battery.ScheduleSlot) error {
	if slot == nil {
		return nil
	}

	_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableBatteryScheduleSlots],
		slot.StartTime(),
		slot.EndTime(),
		slot.ChargePower(),
		slot.PredictedSoC(),
		slot.PricePerKwh(),
		slot.ChargingSource().String(),
	)
	if err != nil {
		log.Errorf("Failed to store battery schedule slot: %v", err)
		return err
	}
	log.Debugf("Stored battery schedule slot starting at %v", slot.StartTime())
	return nil
}

type BatteryScheduleSlotValueChangeListener struct {
	repo *timescaleRepository
}

func (bssvcl *BatteryScheduleSlotValueChangeListener) HandleEvent(event *events.BatteryScheduleSlotEvent) {
	if !event.Valid() {
		if log.WarningEnabled() {
			log.Warning("Not storing battery schedule slot as event is invalid")
		}
		return
	}
	err := bssvcl.repo.storeScheduleSlot(event.Slot())
	if err != nil {
		if log.WarningEnabled() {
			log.Warningf("Unable to store battery schedule slot: %v", err)
		}
	}
}

// LatestScheduleSlot retrieves the most recently starting schedule slot
func (t *timescaleRepository) LatestScheduleSlot() (*repository.BatteryScheduleSlotRecord, error) {
	tdSlots := t.tableDefinitions[tableBatteryScheduleSlots]
	statement, err := sql.NewSelect(tableBatteryScheduleSlots).
		WithColumns(sql.NewColumns(tdSlots.ColumnNames()...)...).
		OrderDescending(sql.NewColumnWithName("start_time")).
		Build()
	if err != nil {
		return nil, err
	}

	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		return t.rowValuesToScheduleSlotRecord(values), nil
	}
	return nil, nil
}

// ScheduleSlotAt retrieves the schedule slot that covers the given time
func (t *timescaleRepository) ScheduleSlotAt(moment time.Time) (*repository.BatteryScheduleSlotRecord, error) {
	tdSlots := t.tableDefinitions[tableBatteryScheduleSlots]
	filter := sql.NewFilterFunction("start_time", sql.LessThanOrEquals, moment).
		And("end_time", sql.GreaterThan, moment)

	statement, err := sql.NewSelect(tableBatteryScheduleSlots).
		WithColumns(sql.NewColumns(tdSlots.ColumnNames()...)...).
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

	if rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		return t.rowValuesToScheduleSlotRecord(values), nil
	}
	return nil, nil
}

// ScheduleSlots retrieves all schedule slots within the given time range
func (t *timescaleRepository) ScheduleSlots(from time.Time, till time.Time) ([]*repository.BatteryScheduleSlotRecord, error) {
	tdSlots := t.tableDefinitions[tableBatteryScheduleSlots]
	filter := t.timeRangeFilter("start_time", from, till)

	statement, err := sql.NewSelect(tableBatteryScheduleSlots).
		WithColumns(sql.NewColumns(tdSlots.ColumnNames()...)...).
		WithFilter(filter).
		OrderAscending(sql.NewColumnWithName("start_time")).
		Build()
	if err != nil {
		return nil, err
	}

	rows, err := t.dbPool.Query(context.Background(), statement.Query, statement.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]*repository.BatteryScheduleSlotRecord, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		slots = append(slots, t.rowValuesToScheduleSlotRecord(values))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return slots, nil
}

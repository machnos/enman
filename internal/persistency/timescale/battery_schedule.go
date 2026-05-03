package timescale

import (
	"context"
	"enman/internal/domain"
	"enman/internal/persistency/sql"
	"time"
)

const tableBatterySchedules = "battery_schedules"

func (t *timescaleRepository) newBatterySchedulesDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableBatterySchedules,
		Columns: []*sql.ColumnDefinition{
			{Name: "time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "battery", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "bucket_size_seconds", SqlType: "INT", Nullable: false},
			{Name: "generated_at", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "action", SqlType: "VARCHAR(40)", Nullable: false},
			{Name: "power_w", SqlType: "DOUBLE PRECISION", Nullable: false},
			{Name: "predicted_soc", SqlType: "DOUBLE PRECISION", Nullable: true},
			{Name: "reason", SqlType: "VARCHAR(200)", Nullable: true},
		},
		PrimaryKey: []string{"time", "battery"},
	}
}

func (t *timescaleRepository) StoreBatterySchedule(record *domain.BatteryScheduleRecord) error {
	if record == nil {
		return nil
	}
	_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableBatterySchedules],
		record.BucketStart,
		record.BatteryName,
		int(record.BucketSize.Seconds()),
		record.GeneratedAt,
		record.Action,
		record.PowerW,
		record.PredictedSoC,
		record.Reason,
	)
	return err
}

func (t *timescaleRepository) BatterySchedules(from time.Time, till time.Time, batteryName string) ([]*domain.BatteryScheduleRecord, error) {
	td := t.tableDefinitions[tableBatterySchedules]
	filter := t.timeRangeFilter("time", from, till)
	if batteryName != "" {
		filter.And("battery", sql.Equals, batteryName)
	}
	stmt, err := sql.NewSelect(tableBatterySchedules).
		WithColumns(sql.NewColumns(td.ColumnNames()...)...).
		WithFilter(filter).
		OrderAscending(sql.NewColumnWithName("time")).
		Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), stmt.Query, stmt.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.BatteryScheduleRecord
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		out = append(out, t.rowValuesToBatterySchedule(values))
	}
	return out, nil
}

func (t *timescaleRepository) BatteryScheduleAtTime(moment time.Time, batteryName string, timeMatchType domain.MatchType) (*domain.BatteryScheduleRecord, error) {
	td := t.tableDefinitions[tableBatterySchedules]
	filter := t.momentFilter("time", moment, timeMatchType)
	if batteryName != "" {
		filter.And("battery", sql.Equals, batteryName)
	}
	selectStatement := sql.NewSelect(tableBatterySchedules).
		WithColumns(sql.NewColumns(td.ColumnNames()...)...).
		WithFilter(filter)
	switch timeMatchType {
	case domain.LessOrEqual:
		selectStatement.OrderDescending(sql.NewColumnWithName("time"))
	case domain.EqualOrGreater:
		selectStatement.OrderAscending(sql.NewColumnWithName("time"))
	}
	stmt, err := selectStatement.Build()
	if err != nil {
		return nil, err
	}
	rows, err := t.dbPool.Query(context.Background(), stmt.Query, stmt.Args...)
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
	return t.rowValuesToBatterySchedule(values), nil
}

func (t *timescaleRepository) rowValuesToBatterySchedule(values []any) *domain.BatteryScheduleRecord {
	predictedSoC := float32(0)
	if values[6] != nil {
		predictedSoC = float32(values[6].(float64))
	}
	reason := ""
	if values[7] != nil {
		reason = values[7].(string)
	}
	return &domain.BatteryScheduleRecord{
		BucketStart:  values[0].(time.Time),
		BatteryName:  values[1].(string),
		BucketSize:   time.Duration(values[2].(int32)) * time.Second,
		GeneratedAt:  values[3].(time.Time),
		Action:       values[4].(string),
		PowerW:       float32(values[5].(float64)),
		PredictedSoC: predictedSoC,
		Reason:       reason,
	}
}

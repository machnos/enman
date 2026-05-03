package timescale

import (
	"context"
	"enman/internal/domain"
	"enman/internal/persistency/sql"
	"time"
)

const tableForecasts = "forecasts"

func (t *timescaleRepository) newForecastsDefinition() *sql.TableDefinition {
	return &sql.TableDefinition{
		Name: tableForecasts,
		Columns: []*sql.ColumnDefinition{
			{Name: "time", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "model", SqlType: "VARCHAR(50)", Nullable: false},
			{Name: "kind", SqlType: "VARCHAR(20)", Nullable: false},
			{Name: "bucket_size_seconds", SqlType: "INT", Nullable: false},
			{Name: "generated_at", SqlType: "TIMESTAMPTZ", Nullable: false},
			{Name: "watts", SqlType: "DOUBLE PRECISION", Nullable: false},
			{Name: "confidence", SqlType: "DOUBLE PRECISION", Nullable: true},
		},
		PrimaryKey: []string{"time", "model", "kind"},
	}
}

func (t *timescaleRepository) StoreForecast(record *domain.ForecastRecord) error {
	if record == nil {
		return nil
	}
	_, err := t.dbPool.Exec(context.Background(), t.insertQueries[tableForecasts],
		record.BucketStart,
		record.ModelName,
		record.Kind,
		int(record.BucketSize.Seconds()),
		record.GeneratedAt,
		record.Watts,
		record.Confidence,
	)
	return err
}

func (t *timescaleRepository) Forecasts(from time.Time, till time.Time, modelName string, kind string) ([]*domain.ForecastRecord, error) {
	td := t.tableDefinitions[tableForecasts]
	filter := t.timeRangeFilter("time", from, till)
	if modelName != "" {
		filter.And("model", sql.Equals, modelName)
	}
	if kind != "" {
		filter.And("kind", sql.Equals, kind)
	}
	stmt, err := sql.NewSelect(tableForecasts).
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
	var out []*domain.ForecastRecord
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		out = append(out, t.rowValuesToForecast(values))
	}
	return out, nil
}

func (t *timescaleRepository) ForecastAtTime(moment time.Time, modelName string, kind string, timeMatchType domain.MatchType) (*domain.ForecastRecord, error) {
	td := t.tableDefinitions[tableForecasts]
	filter := t.momentFilter("time", moment, timeMatchType)
	if modelName != "" {
		filter.And("model", sql.Equals, modelName)
	}
	if kind != "" {
		filter.And("kind", sql.Equals, kind)
	}
	selectStatement := sql.NewSelect(tableForecasts).
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
	return t.rowValuesToForecast(values), nil
}

func (t *timescaleRepository) rowValuesToForecast(values []any) *domain.ForecastRecord {
	confidence := float32(0)
	if values[6] != nil {
		confidence = float32(values[6].(float64))
	}
	return &domain.ForecastRecord{
		BucketStart: values[0].(time.Time),
		ModelName:   values[1].(string),
		Kind:        values[2].(string),
		BucketSize:  time.Duration(values[3].(int32)) * time.Second,
		GeneratedAt: values[4].(time.Time),
		Watts:       float32(values[5].(float64)),
		Confidence:  confidence,
	}
}

package influx

import (
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/query"
	"time"
)

const (
	fieldWaterConsumed = "water_consumed"
	bucketWater        = "water"
)

func (i *influxRepository) WaterSourceNames(from time.Time, till time.Time) ([]string, error) {
	builder := NewQueryBuilder(NewSchemaTagValuesQuery(bucketWater, tagName).SetFrom(from).SetTill(till))
	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	var names []string
	for result.Next() {
		names = append(names, fmt.Sprintf("%v", result.Record().Value()))
	}
	return names, nil
}

func (i *influxRepository) WaterUsages(
	from time.Time,
	till time.Time,
	name string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.WaterUsageRecord, error) {

	builder := NewQueryBuilder(NewBucketQuerySource(bucketWater)).Append(NewRangeStatement(from, till)).
		Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementUsage)))
	if name != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(tagName, Equals, name)))
	}
	builder.Append(i.toAggregateWindowStatement(aggregate)).
		Append(NewPivotStatement("_field", "_time", "_value"))

	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	waterUsages := make([]*domain.WaterUsageRecord, 0)
	for result.Next() {
		waterUsages = append(waterUsages, i.newWaterUsageFromRecord(result.Record()))
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return waterUsages, nil
}

func (i *influxRepository) WaterUsageAtTime(moment time.Time, sourceName string, role domain.EnergySourceRole, timeMatchType domain.MatchType) (*domain.WaterUsageRecord, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketWater))
	if domain.LessOrEqual == timeMatchType {
		// Stop time is excluded, so we need to add the minimum amount of time.
		builder.Append(NewRangeStatement(moment.Add(time.Hour*-1), moment.Add(time.Nanosecond)))
	} else if domain.EqualOrGreater == timeMatchType {
		builder.Append(NewRangeStatement(moment, time.Time{}))
	} else {
		builder.Append(NewRangeStatement(moment, moment.Add(time.Nanosecond)))
	}
	builder.Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementUsage)))
	if sourceName != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(tagName, Equals, sourceName)))
	}
	if role != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(tagRole, Equals, string(role))))
	}
	if domain.LessOrEqual == timeMatchType {
		builder.Append(NewSortStatement("_time").SetDescending())
	} else if domain.EqualOrGreater == timeMatchType {
		builder.Append(NewSortStatement("_time").SetAscending())
	}
	builder.Append(NewPivotStatement("_field", "_time", "_value")).
		Append(NewLimitStatement(1))

	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	hasNext := result.Next()
	if result.Err() != nil {
		return nil, result.Err()
	}
	if !hasNext {
		return nil, nil
	}
	return i.newWaterUsageFromRecord(result.Record()), nil
}

func (i *influxRepository) newWaterUsageFromRecord(record *query.FluxRecord) *domain.WaterUsageRecord {
	waterUsage := &domain.WaterUsageRecord{
		Time:       record.Time(),
		Name:       record.ValueByKey(tagName).(string),
		Role:       record.ValueByKey(tagRole).(string),
		WaterUsage: domain.NewWaterUsage(),
	}
	val := record.ValueByKey(fieldWaterConsumed)
	if val != nil {
		waterUsage.SetWaterConsumed(val.(float64))
	}
	return waterUsage
}

type WaterMeterValueChangeListener struct {
	repo *influxRepository
}

func (wmvcl *WaterMeterValueChangeListener) HandleEvent(values *domain.WaterMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing water meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.WaterUsage() == nil || values.WaterUsage().IsZero() {
		// No usable values in event.
		return
	}
	tags := map[string]string{
		tagName: values.Name(),
		tagRole: string(values.Role()),
	}
	fields := map[string]interface{}{}
	fields[fieldWaterConsumed] = values.WaterUsage().WaterConsumed()
	point := influxdb2.NewPoint(
		measurementUsage,
		tags,
		fields,
		values.EventTime())

	wmvcl.repo.writeApis[bucketWater].WritePoint(point)
}

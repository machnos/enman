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
	fieldSoC      = "soc"
	fieldSoH      = "soh"
	bucketBattery = "battery"
)

func (i *influxRepository) BatterySourceNames(from time.Time, till time.Time) ([]string, error) {
	builder := NewQueryBuilder(NewSchemaTagValuesQuery(bucketBattery, tagName).SetFrom(from).SetTill(till))
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

func (i *influxRepository) BatteryStates(
	from time.Time,
	till time.Time,
	name string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.BatteryStateRecord, error) {

	builder := NewQueryBuilder(NewBucketQuerySource(bucketBattery)).Append(NewRangeStatement(from, till)).
		Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementState)))
	if name != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(tagName, Equals, name)))
	}
	builder.Append(i.toAggregateWindowStatement(aggregate)).
		Append(NewPivotStatement("_field", "_time", "_value"))

	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	batteryStates := make([]*domain.BatteryStateRecord, 0)
	for result.Next() {
		batteryStates = append(batteryStates, i.newBatteryStateFromRecord(result.Record()))
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return batteryStates, nil
}

func (i *influxRepository) BatteryStateAtTime(moment time.Time, sourceName string, role domain.EnergySourceRole, timeMatchType domain.MatchType) (*domain.BatteryStateRecord, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketBattery))
	if domain.LessOrEqual == timeMatchType {
		// Stop time is excluded, so we need to add the minimum amount of time.
		builder.Append(NewRangeStatement(moment.Add(time.Hour*-1), moment.Add(time.Nanosecond)))
	} else if domain.EqualOrGreater == timeMatchType {
		builder.Append(NewRangeStatement(moment, time.Time{}))
	} else {
		builder.Append(NewRangeStatement(moment, moment.Add(time.Nanosecond)))
	}
	builder.Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementState)))
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
	return i.newBatteryStateFromRecord(result.Record()), nil
}

func (i *influxRepository) newBatteryStateFromRecord(record *query.FluxRecord) *domain.BatteryStateRecord {
	batteryState := &domain.BatteryStateRecord{
		Time:         record.Time(),
		Name:         record.ValueByKey(tagName).(string),
		Role:         record.ValueByKey(tagRole).(string),
		BatteryState: domain.NewBatteryState(),
	}
	val := record.ValueByKey(fieldSuffixCurrent)
	if val != nil {
		batteryState.SetCurrent(float32(val.(float64)))
	}
	val = record.ValueByKey(fieldSuffixPower)
	if val != nil {
		batteryState.SetPower(float32(val.(float64)))
	}
	val = record.ValueByKey(fieldSuffixVoltage)
	if val != nil {
		batteryState.SetVoltage(float32(val.(float64)))
	}
	val = record.ValueByKey(fieldSoC)
	if val != nil {
		batteryState.SetSoC(float32(val.(float64)))
	}
	val = record.ValueByKey(fieldSoH)
	if val != nil {
		batteryState.SetSoH(float32(val.(float64)))
	}
	return batteryState
}

type BatteryMeterValueChangeListener struct {
	repo *influxRepository
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
	tags := map[string]string{
		tagName: values.Name(),
		tagRole: string(values.Role()),
	}
	fields := map[string]interface{}{}
	fields[fieldSuffixCurrent] = values.BatteryState().Current()
	fields[fieldSuffixPower] = values.BatteryState().Power()
	fields[fieldSuffixVoltage] = values.BatteryState().Voltage()
	fields[fieldSoC] = values.BatteryState().SoC()
	fields[fieldSoH] = values.BatteryState().SoH()
	point := influxdb2.NewPoint(
		measurementState,
		tags,
		fields,
		values.EventTime())

	bmvcl.repo.writeApis[bucketBattery].WritePoint(point)
}

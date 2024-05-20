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
	fieldPrefixLine                              = "l%d_"
	fieldPrefixTotal                             = "total_"
	fieldPrefixConsumption                       = "consumption_"
	fieldPrefixFeedback                          = "feedback_"
	fieldSuffixCurrent                           = "current"
	fieldSuffixCosts                             = "costs"
	fieldSuffixEnergyConsumed                    = "energy_consumed"
	fieldSuffixEnergyProvided                    = "energy_provided"
	fieldSuffixPower                             = "power"
	fieldSuffixPricePerKwh                       = "price_per_kwh"
	fieldSuffixVoltage                           = "voltage"
	bucketElectricity                            = "electricity"
	bucketElectricityFieldTotalCurrent           = fieldPrefixTotal + fieldSuffixCurrent
	bucketElectricityFieldTotalEnergyConsumed    = fieldPrefixTotal + fieldSuffixEnergyConsumed
	bucketElectricityFieldTotalEnergyProvided    = fieldPrefixTotal + fieldSuffixEnergyProvided
	bucketElectricityFieldTotalPower             = fieldPrefixTotal + fieldSuffixPower
	bucketElectricityFieldConsumptionPricePerKwh = fieldPrefixConsumption + fieldSuffixPricePerKwh
	bucketElectricityFieldConsumptionCosts       = fieldPrefixConsumption + fieldSuffixCosts
	bucketElectricityFieldFeedbackPricePerKwh    = fieldPrefixFeedback + fieldSuffixPricePerKwh
	bucketElectricityFieldFeedbackCosts          = fieldPrefixFeedback + fieldSuffixCosts
	measurementState                             = "state"
	measurementCosts                             = "costs"
)

func (i *influxRepository) ElectricitySourceNames(from time.Time, till time.Time) ([]string, error) {
	builder := NewQueryBuilder(NewSchemaTagValuesQuery(bucketElectricity, tagName).SetFrom(from).SetTill(till))
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

func (i *influxRepository) ElectricityUsages(
	from time.Time,
	till time.Time,
	name string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.ElectricityUsageRecord, error) {

	builder := NewQueryBuilder(NewBucketQuerySource(bucketElectricity)).Append(NewRangeStatement(from, till)).
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
	energyUsages := make([]*domain.ElectricityUsageRecord, 0)
	for result.Next() {
		energyUsages = append(energyUsages, i.newElectricityUsageFromRecord(result.Record()))
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return energyUsages, nil
}

func (i *influxRepository) ElectricityUsageAtTime(moment time.Time, sourceName string, role domain.EnergySourceRole, timeMatchType domain.MatchType) (*domain.ElectricityUsageRecord, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketElectricity))
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
	return i.newElectricityUsageFromRecord(result.Record()), nil
}

func (i *influxRepository) ElectricityStates(
	from time.Time,
	till time.Time,
	name string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.ElectricityStateRecord, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketElectricity)).Append(NewRangeStatement(from, till)).
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
	states := make([]*domain.ElectricityStateRecord, 0)
	for result.Next() {
		state := &domain.ElectricityStateRecord{
			Time:             result.Record().Time(),
			Name:             result.Record().ValueByKey(tagName).(string),
			Role:             result.Record().ValueByKey(tagRole).(string),
			ElectricityState: domain.NewElectricityState(),
		}
		for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
			val := result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixCurrent, lineIx+1))
			if val != nil {
				state.SetCurrent(lineIx, float32(val.(float64)))
			}
			val = result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixPower, lineIx+1))
			if val != nil {
				state.SetPower(lineIx, float32(val.(float64)))
			}
			val = result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixVoltage, lineIx+1))
			if val != nil {
				state.SetVoltage(lineIx, float32(val.(float64)))
			}
		}
		states = append(states, state)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return states, nil
}

func (i *influxRepository) ElectricityCosts(
	from time.Time,
	till time.Time,
	name string,
	aggregate *domain.AggregateConfiguration,
) ([]*domain.ElectricityCostRecord, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketElectricity)).Append(NewRangeStatement(from, till)).
		Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementCosts)))
	if name != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(tagName, Equals, name)))
	}
	// Shift the records by -1m because the stored time is the end time of the cost calculation.
	// For example a time of 14:00 means the energy cost between 13:00-14:00. By setting the time to 13:59 it will be
	// counted in the 13:00-14:00 aggregate with a time of 14:00. It's up to the GUI to make sure 14:00 is the end time.
	builder.Append(NewTimeShiftStatement("-1m")).
		Append(i.toAggregateWindowStatement(aggregate)).
		Append(NewPivotStatement("_field", "_time", "_value"))

	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	costs := make([]*domain.ElectricityCostRecord, 0)
	for result.Next() {
		cost := &domain.ElectricityCostRecord{
			Time:                   result.Record().Time(),
			Name:                   result.Record().ValueByKey(tagName).(string),
			ConsumptionCosts:       float32(result.Record().ValueByKey(bucketElectricityFieldConsumptionCosts).(float64)),
			ConsumptionPricePerKwh: float32(result.Record().ValueByKey(bucketElectricityFieldConsumptionPricePerKwh).(float64)),
			ConsumptionEnergy:      float32(result.Record().ValueByKey(bucketElectricityFieldTotalEnergyConsumed).(float64)),
			FeedbackCosts:          float32(result.Record().ValueByKey(bucketElectricityFieldFeedbackCosts).(float64)),
			FeedbackPricePerKwh:    float32(result.Record().ValueByKey(bucketElectricityFieldFeedbackPricePerKwh).(float64)),
			FeedbackEnergy:         float32(result.Record().ValueByKey(bucketElectricityFieldTotalEnergyProvided).(float64)),
		}
		costs = append(costs, cost)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return costs, nil
}

func (i *influxRepository) newElectricityUsageFromRecord(record *query.FluxRecord) *domain.ElectricityUsageRecord {
	electricityUsage := &domain.ElectricityUsageRecord{
		Time:             record.Time(),
		Name:             record.ValueByKey(tagName).(string),
		Role:             record.ValueByKey(tagRole).(string),
		ElectricityUsage: domain.NewElectricityUsage(),
	}
	for lineIx := uint8(0); lineIx < domain.MaxPhases; lineIx++ {
		val := record.ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyConsumed, lineIx+1))
		if val != nil {
			electricityUsage.SetEnergyConsumed(lineIx, val.(float64))
		}
		val = record.ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyProvided, lineIx+1))
		if val != nil {
			electricityUsage.SetEnergyProvided(lineIx, val.(float64))
		}
	}

	electricityUsage.ElectricityUsage.SetTotalEnergyConsumed(record.ValueByKey(bucketElectricityFieldTotalEnergyConsumed).(float64))
	electricityUsage.ElectricityUsage.SetTotalEnergyProvided(record.ValueByKey(bucketElectricityFieldTotalEnergyProvided).(float64))
	return electricityUsage
}

type ElectricityMeterValueChangeListener struct {
	repo *influxRepository
}

func (emvcl *ElectricityMeterValueChangeListener) HandleEvent(values *domain.ElectricityMeterValues) {
	valid, err := values.Valid()
	if !valid {
		if log.WarningEnabled() {
			log.Warningf("Not storing electricity meter reading from '%s' as it is invalid: %v", values.Name(), err)
		}
	}
	if values.ElectricityState() == nil && values.ElectricityUsage() == nil {
		// No usable values in event.
		return
	}
	tags := map[string]string{
		tagName: values.Name(),
		tagRole: string(values.Role()),
	}
	if values.ElectricityState() != nil {
		fields := map[string]interface{}{}
		fields[bucketElectricityFieldTotalCurrent] = values.ElectricityState().TotalCurrent()
		fields[bucketElectricityFieldTotalPower] = values.ElectricityState().TotalPower()
		for _, lineIx := range values.ReadLineIndices() {
			fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixCurrent, lineIx+1)] = values.ElectricityState().Current(lineIx)
			fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixPower, lineIx+1)] = values.ElectricityState().Power(lineIx)
			fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixVoltage, lineIx+1)] = values.ElectricityState().Voltage(lineIx)
		}
		point := influxdb2.NewPoint(
			measurementState,
			tags,
			fields,
			values.EventTime())

		emvcl.repo.writeApis[bucketElectricity].WritePoint(point)
	}
	if values.ElectricityUsage() != nil {
		fields := map[string]interface{}{}
		fields[bucketElectricityFieldTotalEnergyConsumed] = values.ElectricityUsage().TotalEnergyConsumed()
		fields[bucketElectricityFieldTotalEnergyProvided] = values.ElectricityUsage().TotalEnergyProvided()
		for _, lineIx := range values.ReadLineIndices() {
			if values.ElectricityUsage().EnergyConsumed(lineIx) != 0 {
				fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyConsumed, lineIx+1)] = values.ElectricityUsage().EnergyConsumed(lineIx)
			}
			if values.ElectricityUsage().EnergyProvided(lineIx) != 0 {
				fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyProvided, lineIx+1)] = values.ElectricityUsage().EnergyProvided(lineIx)
			}
		}
		point := influxdb2.NewPoint(
			measurementUsage,
			tags,
			fields,
			values.EventTime())

		emvcl.repo.writeApis[bucketElectricity].WritePoint(point)
	}
}

type ElectricityCostsValueChangeListener struct {
	repo *influxRepository
}

func (ecvcl *ElectricityCostsValueChangeListener) HandleEvent(values *domain.ElectricityCostsValues) {
	tags := map[string]string{
		tagName: values.EnergyProviderName(),
		tagRole: string(domain.RoleGrid),
	}
	fields := map[string]interface{}{
		bucketElectricityFieldTotalEnergyConsumed:    values.ConsumptionEnergy(),
		bucketElectricityFieldConsumptionPricePerKwh: values.ConsumptionPricePerKwh(),
		bucketElectricityFieldConsumptionCosts:       values.ConsumptionCosts(),
		bucketElectricityFieldTotalEnergyProvided:    values.FeedbackEnergy(),
		bucketElectricityFieldFeedbackPricePerKwh:    values.FeedbackPricePerKwh(),
		bucketElectricityFieldFeedbackCosts:          values.FeedbackCosts(),
	}

	point := influxdb2.NewPoint(
		measurementCosts,
		tags,
		fields,
		values.EndTime())

	ecvcl.repo.writeApis[bucketElectricity].WritePoint(point)
}

package influx

import (
	"context"
	"enman/internal/domain"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/query"
	"math"
	"time"
)

const (
	bucketPrices                      = "prices"
	bucketPricesFieldConsumptionPrice = "consumption_price"
	bucketPricesFieldFeedbackPrice    = "feedback_price"
	bucketPricesTagsProvider          = "provider"
	measurementEnergyPrice            = "energy_price"
)

func (i *influxRepository) EnergyPriceProviderNames(from time.Time, till time.Time) ([]string, error) {
	builder := NewQueryBuilder(NewSchemaTagValuesQuery(bucketPrices, bucketPricesTagsProvider).SetFrom(from).SetTill(till))
	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	var providers []string
	for result.Next() {
		providers = append(providers, fmt.Sprintf("%v", result.Record().Value()))
	}
	return providers, nil
}

func (i *influxRepository) EnergyPrices(from time.Time, till time.Time, provider string) ([]*domain.EnergyPrice, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketPrices)).
		Append(NewRangeStatement(from, till))
	if provider != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(bucketPricesTagsProvider, Equals, provider)))
	}
	builder.Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementEnergyPrice))).
		Append(NewPivotStatement("_field", "_time", "_value"))
	result, err := i.queryApi.Query(context.Background(), builder.Build())
	if err != nil {
		return nil, err
	}
	energyPrices := make([]*domain.EnergyPrice, 0)
	for result.Next() {
		price := i.NewEnergyPriceFromRecord(result.Record())
		energyPrices = append(energyPrices, price)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return energyPrices, nil
}

func (i *influxRepository) EnergyPriceAtTime(moment time.Time, providerName string, timeMatchType domain.MatchType) (*domain.EnergyPrice, error) {
	builder := NewQueryBuilder(NewBucketQuerySource(bucketPrices))
	if domain.LessOrEqual == timeMatchType {
		// Stop time is excluded, so we need to add the minimum amount of time.
		builder.Append(NewRangeStatement(moment.Add(time.Hour*-2), moment.Add(time.Nanosecond)))
	} else if domain.EqualOrGreater == timeMatchType {
		builder.Append(NewRangeStatement(moment, time.Time{}))
	} else {
		builder.Append(NewRangeStatement(moment, moment.Add(time.Nanosecond)))
	}
	builder.Append(NewFilterStatement(NewFilterFunction("_measurement", Equals, measurementEnergyPrice)))
	if providerName != "" {
		builder.Append(NewFilterStatement(NewFilterFunction(bucketPricesTagsProvider, Equals, providerName)))
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
	return i.NewEnergyPriceFromRecord(result.Record()), nil
}

func (i *influxRepository) StoreEnergyPrice(price *domain.EnergyPrice) {
	fields := map[string]interface{}{
		bucketPricesFieldConsumptionPrice: math.Ceil(float64(price.ConsumptionPrice)*100000) / 100000,
		bucketPricesFieldFeedbackPrice:    math.Ceil(float64(price.FeedbackPrice)*100000) / 100000,
	}
	tags := map[string]string{
		bucketPricesTagsProvider: price.Provider,
	}
	point := influxdb2.NewPoint(
		measurementEnergyPrice,
		tags,
		fields,
		price.Time)
	i.writeApis[bucketPrices].WritePoint(point)
}

func (i *influxRepository) NewEnergyPriceFromRecord(record *query.FluxRecord) *domain.EnergyPrice {
	return &domain.EnergyPrice{
		Time:             record.Time(),
		ConsumptionPrice: float32(record.ValueByKey(bucketPricesFieldConsumptionPrice).(float64)),
		FeedbackPrice:    float32(record.ValueByKey(bucketPricesFieldFeedbackPrice).(float64)),
		Provider:         record.ValueByKey(bucketPricesTagsProvider).(string),
	}
}

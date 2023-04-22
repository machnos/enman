package influx

import (
	"context"
	"enman/internal/prices"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"math"
	"time"
)

const (
	bucketPrices             = "prices"
	bucketPricesFieldPrice   = "price"
	bucketPricesTagsProvider = "provider"
	measurementEnergyPrice   = "energy_price"
)

func (i *influxRepository) EnergyPriceProviders() ([]string, error) {
	query := fmt.Sprintf(`from(bucket: "%s")
				|> range(start: %s)
				|> group(columns: ["%s"])
				|> distinct(column: "%s")
                |> keep(columns: ["_value"])
                |> filter(fn: (r) => r._value != "")`, bucketPrices, "-1y", bucketPricesTagsProvider, bucketPricesTagsProvider)
	result, err := i.queryApi.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	var providers []string
	for result.Next() {
		providers = append(providers, fmt.Sprintf("%v", result.Record().Value()))
	}
	return providers, nil
}

func (i *influxRepository) EnergyPrices(from *time.Time, till *time.Time, provider string) ([]*prices.EnergyPrice, error) {
	queryRange := ""
	if till != nil {
		queryRange = fmt.Sprintf("start: %d, stop: %d", from.Unix(), till.Unix())
	} else {
		queryRange = fmt.Sprintf("start: %d", from.Unix())
	}
	providerFilter := ""
	if provider != "" {
		providerFilter = fmt.Sprintf("|> filter(fn: (r) => r[\"%s\"] == \"%s\")", bucketPricesTagsProvider, provider)
	}
	query := fmt.Sprintf(`from(bucket: "%s")
				|> range(%s)
				|> filter(fn: (r) => r._measurement == "%s")
                |> filter(fn: (r) => r._field == "%s")
				%s`, bucketPrices, queryRange, measurementEnergyPrice, bucketPricesFieldPrice, providerFilter)
	result, err := i.queryApi.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	energyPrices := make([]*prices.EnergyPrice, 0)
	for result.Next() {
		price := &prices.EnergyPrice{
			Time:     result.Record().Time(),
			Price:    float32(result.Record().Value().(float64)),
			Provider: result.Record().ValueByKey(bucketPricesTagsProvider).(string),
		}
		energyPrices = append(energyPrices, price)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return energyPrices, nil
}

func (i *influxRepository) StoreEnergyPrice(price *prices.EnergyPrice) {
	fields := map[string]interface{}{
		bucketPricesFieldPrice: math.Ceil(float64(price.Price)*100000) / 100000,
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

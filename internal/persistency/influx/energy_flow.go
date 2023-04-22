package influx

import (
	"context"
	"enman/internal/energysource"
	"enman/internal/persistency"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"time"
)

const (
	fieldPrefixLine                          = "l%d_"
	fieldPrefixTotal                         = "total_"
	fieldSuffixCurrent                       = "current"
	fieldSuffixEnergyConsumed                = "energy_consumed"
	fieldSuffixEnergyProvided                = "energy_provided"
	fieldSuffixPower                         = "power"
	fieldSuffixVoltage                       = "voltage"
	bucketEnergyFlow                         = "energy_flow"
	bucketEnergyFlowFieldTotalCurrent        = fieldPrefixTotal + fieldSuffixCurrent
	bucketEnergyFlowFieldTotalEnergyConsumed = fieldPrefixTotal + fieldSuffixEnergyConsumed
	bucketEnergyFlowFieldTotalEnergyProvided = fieldPrefixTotal + fieldSuffixEnergyProvided
	bucketEnergyFlowFieldTotalPower          = fieldPrefixTotal + fieldSuffixPower
	bucketEnergyFlowTagName                  = "name"
	bucketEnergyFlowTagRole                  = "role"
	measurementEnergyUsage                   = "energy_usage"
)

func (i *influxRepository) EnergyFlowNames() ([]string, error) {
	query := fmt.Sprintf(`from(bucket: "%s")
				|> range(start: %s)
				|> group(columns: ["%s"])
				|> distinct(column: "%s")
                |> keep(columns: ["_value"])
                |> filter(fn: (r) => r._value != "")`, bucketEnergyFlow, "-1y", bucketEnergyFlowTagName, bucketEnergyFlowTagName)
	result, err := i.queryApi.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	var roles []string
	for result.Next() {
		roles = append(roles, fmt.Sprintf("%v", result.Record().Value()))
	}
	return roles, nil
}

func (i *influxRepository) EnergyFlowUsages(from *time.Time, till *time.Time, name string) ([]*persistency.EnergyFlowUsage, error) {
	queryRange := ""
	if till != nil {
		queryRange = fmt.Sprintf("start: %d, stop: %d", from.Unix(), till.Unix())
	} else {
		queryRange = fmt.Sprintf("start: %d", from.Unix())
	}
	nameFilter := ""
	if name != "" {
		nameFilter = fmt.Sprintf("|> filter(fn: (r) => r[\"%s\"] == \"%s\")", bucketEnergyFlowTagName, name)
	}
	query := fmt.Sprintf(`from(bucket: "%s")
				|> range(%s)
				|> filter(fn: (r) => r._measurement == "%s")
                %s
                |> filter(fn: (r) => r["_field"] =~ /energy_consumed|energy_provided/)
                |> aggregateWindow(every: 1h, fn: max, createEmpty: false)
                |> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")`,
		bucketEnergyFlow, queryRange, measurementEnergyUsage, nameFilter)
	result, err := i.queryApi.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	energyFlows := make([]*persistency.EnergyFlowUsage, 0)
	for result.Next() {
		energyFlow := &persistency.EnergyFlowUsage{
			Time:            result.Record().Time(),
			Name:            result.Record().ValueByKey(bucketEnergyFlowTagName).(string),
			Role:            result.Record().ValueByKey(bucketEnergyFlowTagRole).(string),
			EnergyFlowUsage: energysource.NewEnergyFlowUsage(),
		}
		energyFlow.EnergyFlowUsage.SetTotalEnergyConsumed(result.Record().ValueByKey(bucketEnergyFlowFieldTotalEnergyConsumed).(float64))
		energyFlow.EnergyFlowUsage.SetTotalEnergyProvided(result.Record().ValueByKey(bucketEnergyFlowFieldTotalEnergyProvided).(float64))
		energyFlows = append(energyFlows, energyFlow)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return energyFlows, nil
}

func (i *influxRepository) EnergyFlowStates(from *time.Time, till *time.Time, name string) ([]*persistency.EnergyFlowState, error) {
	queryRange := ""
	if till != nil {
		queryRange = fmt.Sprintf("start: %d, stop: %d", from.Unix(), till.Unix())
	} else {
		queryRange = fmt.Sprintf("start: %d", from.Unix())
	}
	nameFilter := ""
	if name != "" {
		nameFilter = fmt.Sprintf("|> filter(fn: (r) => r[\"%s\"] == \"%s\")", bucketEnergyFlowTagName, name)
	}
	query := fmt.Sprintf(`from(bucket: "%s")
				|> range(%s)
				|> filter(fn: (r) => r._measurement == "%s")
                %s
               |> aggregateWindow(every: 1m, fn: mean, createEmpty: false)
                |> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")`,
		bucketEnergyFlow, queryRange, measurementEnergyUsage, nameFilter)
	result, err := i.queryApi.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	states := make([]*persistency.EnergyFlowState, 0)
	for result.Next() {
		state := &persistency.EnergyFlowState{
			Time:            result.Record().Time(),
			Name:            result.Record().ValueByKey(bucketEnergyFlowTagName).(string),
			Role:            result.Record().ValueByKey(bucketEnergyFlowTagRole).(string),
			EnergyFlowState: energysource.NewEnergyFlowState(),
		}
		for lineIx := uint8(0); lineIx < energysource.MaxPhases; lineIx++ {
			val := result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixCurrent, lineIx+1))
			if val != nil {
				_, _ = state.SetCurrent(lineIx, float32(val.(float64)))
			}
			val = result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixPower, lineIx+1))
			if val != nil {
				_, _ = state.SetPower(lineIx, float32(val.(float64)))
			}
			val = result.Record().ValueByKey(fmt.Sprintf(fieldPrefixLine+fieldSuffixVoltage, lineIx+1))
			if val != nil {
				_, _ = state.SetVoltage(lineIx, float32(val.(float64)))
			}
		}
		states = append(states, state)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return states, nil
}

func (i *influxRepository) StoreEnergyFlow(energyFlow energysource.EnergyFlow) {
	fields := map[string]interface{}{
		bucketEnergyFlowFieldTotalCurrent:        energyFlow.TotalCurrent(),
		bucketEnergyFlowFieldTotalEnergyConsumed: energyFlow.TotalEnergyConsumed(),
		bucketEnergyFlowFieldTotalEnergyProvided: energyFlow.TotalEnergyProvided(),
		bucketEnergyFlowFieldTotalPower:          energyFlow.TotalPower(),
	}
	for ix := uint8(0); ix < energyFlow.Phases(); ix++ {
		fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixCurrent, ix+1)] = energyFlow.Current(ix)
		if energyFlow.EnergyConsumed(ix) != 0 {
			fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyConsumed, ix+1)] = energyFlow.EnergyConsumed(ix)
		}
		if energyFlow.EnergyProvided(ix) != 0 {
			fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixEnergyProvided, ix+1)] = energyFlow.EnergyProvided(ix)
		}
		fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixPower, ix+1)] = energyFlow.Power(ix)
		fields[fmt.Sprintf(fieldPrefixLine+fieldSuffixVoltage, ix+1)] = energyFlow.Voltage(ix)
	}
	tags := map[string]string{
		bucketEnergyFlowTagRole: energyFlow.Role(),
	}
	if energyFlow.Name() != "" {
		tags[bucketEnergyFlowTagName] = energyFlow.Name()
	}
	point := influxdb2.NewPoint(
		measurementEnergyUsage,
		tags,
		fields,
		time.Now())

	i.writeApis[bucketEnergyFlow].WritePoint(point)
}

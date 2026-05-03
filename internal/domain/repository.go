package domain

import (
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/gas"
	"enman/internal/domain/prices"
	"enman/internal/domain/water"
	"fmt"
	"math"
	"strings"
	"time"
)

type MatchType uint8

const (
	Equal MatchType = iota
	LessOrEqual
	EqualOrGreater
)

type WindowUnit uint64

const (
	WindowUnitNanosecond = iota
	WindowUnitMicrosecond
	WindowUnitMillisecond
	WindowUnitSecond
	WindowUnitMinute
	WindowUnitHour
	WindowUnitDay
	WindowUnitWeek
	WindowUnitMonth
	WindowUnitYear
)

func (u WindowUnit) String() string {
	switch u {
	case WindowUnitNanosecond:
		return "nanosecond"
	case WindowUnitMicrosecond:
		return "microsecond"
	case WindowUnitMillisecond:
		return "millisecond"
	case WindowUnitSecond:
		return "second"
	case WindowUnitMinute:
		return "minute"
	case WindowUnitHour:
		return "hour"
	case WindowUnitDay:
		return "day"
	case WindowUnitWeek:
		return "week"
	case WindowUnitMonth:
		return "month"
	case WindowUnitYear:
		return "year"
	default:
		return fmt.Sprintf("unknown WindowUnit (%d)", u)
	}
}

func ParseWindowUnit(windowUnit string) (WindowUnit, error) {
	switch strings.ToLower(windowUnit) {
	case "nanosecond":
		return WindowUnitNanosecond, nil
	case "microsecond":
		return WindowUnitMicrosecond, nil
	case "millisecond":
		return WindowUnitMillisecond, nil
	case "second":
		return WindowUnitSecond, nil
	case "minute":
		return WindowUnitMinute, nil
	case "hour":
		return WindowUnitHour, nil
	case "day":
		return WindowUnitDay, nil
	case "week":
		return WindowUnitWeek, nil
	case "month":
		return WindowUnitMonth, nil
	case "year":
		return WindowUnitYear, nil
	default:
		return WindowUnit(math.MaxUint64), fmt.Errorf("invalid WindowUnit: %s", windowUnit)
	}
}

type AggregateFunction uint64

const (
	AggregateFunctionCount = iota
	AggregateFunctionMax
	AggregateFunctionMean
	AggregateFunctionMedian
	AggregateFunctionMin
	AggregateFunctionSum
)

func (f AggregateFunction) String() string {
	switch f {
	case AggregateFunctionCount:
		return "count"
	case AggregateFunctionMax:
		return "max"
	case AggregateFunctionMean:
		return "mean"
	case AggregateFunctionMedian:
		return "median"
	case AggregateFunctionMin:
		return "min"
	case AggregateFunctionSum:
		return "sum"
	default:
		return fmt.Sprintf("unknown AggregateFunction (%d)", f)
	}
}

func ParseAggregateFunction(aggregateFunction string) (AggregateFunction, error) {
	switch strings.ToLower(aggregateFunction) {
	case "count":
		return AggregateFunctionCount, nil
	case "max":
		return AggregateFunctionMax, nil
	case "mean":
		return AggregateFunctionMean, nil
	case "median":
		return AggregateFunctionMedian, nil
	case "min":
		return AggregateFunctionMin, nil
	case "sum":
		return AggregateFunctionSum, nil
	default:
		return AggregateFunction(math.MaxUint64), fmt.Errorf("invalid AggregateFunction: %s", aggregateFunction)
	}
}

type Repository interface {
	ElectricitySourceNames(from time.Time, till time.Time) ([]string, error)
	ElectricityUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityUsagesRecord, error)
	ElectricityUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*ElectricityUsageRecord, error)
	ElectricityStates(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityStatesRecord, error)
	ElectricityCosts(from time.Time, till time.Time, providerName string, aggregate *AggregateConfiguration) ([]*ElectricityCostsRecord, error)

	EnergyPriceProviders(from time.Time, till time.Time) ([]*prices.EnergyPriceProvider, error)
	EnergyPrices(from time.Time, till time.Time, providerName string, energyType prices.EnergyType) ([]*prices.EnergyPrice, error)
	EnergyPriceAtTime(moment time.Time, providerName string, energyType prices.EnergyType, timeMatchType MatchType) (*prices.EnergyPrice, error)
	StoreEnergyPrice(price *prices.EnergyPrice) error

	GasSourceNames(from time.Time, till time.Time) ([]string, error)
	GasUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*GasUsagesRecord, error)
	GasUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*GasUsageRecord, error)
	GasCosts(from time.Time, till time.Time, providerName string, aggregate *AggregateConfiguration) ([]*GasCostsRecord, error)

	WaterSourceNames(from time.Time, till time.Time) ([]string, error)
	WaterUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*WaterUsagesRecord, error)
	WaterUsageAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*WaterUsageRecord, error)

	BatterySourceNames(from time.Time, till time.Time) ([]string, error)
	BatteryStates(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*BatteryStatesRecord, error)
	BatteryStateAtTime(moment time.Time, sourceName string, role constants.EnergySourceRole, timeMatchType MatchType) (*BatteryStateRecord, error)

	StoreForecast(record *ForecastRecord) error
	Forecasts(from time.Time, till time.Time, modelName string, kind string, sourceName string) ([]*ForecastRecord, error)
	ForecastAtTime(moment time.Time, modelName string, kind string, sourceName string, timeMatchType MatchType) (*ForecastRecord, error)

	Initialize() error
	Close()
}

func AggregateFunctionsOf(functions string) ([]AggregateFunction, error) {
	result := make([]AggregateFunction, 0)
	fn := strings.Split(functions, ",")
	for _, fnName := range fn {
		function, err := ParseAggregateFunction(fnName)
		if err != nil {
			return nil, err
		}
		result = append(result, function)
	}
	return result, nil
}

type AggregateConfiguration struct {
	WindowUnit   WindowUnit
	WindowAmount uint64
	Functions    []AggregateFunction
	CreateEmpty  bool
}

type ElectricityUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*electricity.Usage
}

type ElectricityUsagesRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Role      string
	Usages    map[AggregateFunction]*electricity.Usage
}

type ElectricityStatesRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Role      string
	States    map[AggregateFunction]*electricity.State
}

type GasUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*gas.Usage
}

type GasUsagesRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Role      string
	Usages    map[AggregateFunction]*gas.Usage
}

type WaterUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*water.Usage
}

type WaterUsagesRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Role      string
	Usages    map[AggregateFunction]*water.Usage
}

type BatteryStateRecord struct {
	Time time.Time
	Name string
	Role string
	*battery.State
}

type BatteryStatesRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Role      string
	States    map[AggregateFunction]*battery.State
}

type ElectricityCostsRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Costs     map[AggregateFunction]*electricity.Costs
}

type GasCostsRecord struct {
	StartTime time.Time
	EndTime   time.Time
	Name      string
	Costs     map[AggregateFunction]*gas.Costs
}

// ForecastRecord is a single forecast bucket persisted in storage.
type ForecastRecord struct {
	BucketStart time.Time
	BucketSize  time.Duration
	GeneratedAt time.Time
	ModelName   string
	Kind        string
	SourceName  string
	Watts       float32
	Confidence  float32
}

// BatteryScheduleRecord is a single planned battery action bucket produced by
// the optimizer. It is a transient in-memory plan output consumed by the
// PriceAwareBatteryForecaster; not persisted on its own.
type BatteryScheduleRecord struct {
	BucketStart  time.Time
	BucketSize   time.Duration
	GeneratedAt  time.Time
	BatteryName  string
	Action       string
	PowerW       float32
	PredictedSoC float32
	Reason       string
}

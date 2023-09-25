package domain

import (
	"fmt"
	"strings"
	"time"
)

type MatchType uint8

const (
	Equal MatchType = iota
	LessOrEqual
	EqualOrGreater
)

type WindowUnit uint8

const (
	WindowUnitNanosecond WindowUnit = iota
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

type Repository interface {
	ElectricitySourceNames(from time.Time, till time.Time) ([]string, error)
	ElectricityUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityUsageRecord, error)
	ElectricityUsageAtTime(moment time.Time, sourceName string, role EnergySourceRole, timeMatchType MatchType) (*ElectricityUsageRecord, error)
	ElectricityStates(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*ElectricityStateRecord, error)
	ElectricityCosts(from time.Time, till time.Time, providerName string, aggregate *AggregateConfiguration) ([]*ElectricityCostRecord, error)

	EnergyPriceProviderNames(from time.Time, till time.Time) ([]string, error)
	EnergyPrices(from time.Time, till time.Time, providerName string) ([]*EnergyPrice, error)
	EnergyPriceAtTime(moment time.Time, providerName string, timeMatchType MatchType) (*EnergyPrice, error)
	StoreEnergyPrice(price *EnergyPrice)

	GasSourceNames(from time.Time, till time.Time) ([]string, error)
	GasUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*GasUsageRecord, error)
	GasUsageAtTime(moment time.Time, sourceName string, role EnergySourceRole, timeMatchType MatchType) (*GasUsageRecord, error)

	WaterSourceNames(from time.Time, till time.Time) ([]string, error)
	WaterUsages(from time.Time, till time.Time, sourceName string, aggregate *AggregateConfiguration) ([]*WaterUsageRecord, error)
	WaterUsageAtTime(moment time.Time, sourceName string, role EnergySourceRole, timeMatchType MatchType) (*WaterUsageRecord, error)

	Initialize() error
	Close()
}

func WindowUnitOf(unit string) (WindowUnit, error) {
	switch strings.ToLower(unit) {
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
	case "Year":
		return WindowUnitYear, nil
	}
	return 0, fmt.Errorf("invalid WindowUnit type %s", unit)
}

type AggregateFunction interface {
}
type Count struct {
	AggregateFunction
}
type Max struct {
	AggregateFunction
}
type Mean struct {
	AggregateFunction
}
type Median struct {
	AggregateFunction
}
type Min struct {
	AggregateFunction
}
type Sum struct {
	AggregateFunction
}

func AggregateFunctionOf(function string) (AggregateFunction, error) {
	switch strings.ToLower(function) {
	case "count":
		return &Count{}, nil
	case "max":
		return &Max{}, nil
	case "mean":
		return &Mean{}, nil
	case "median":
		return &Median{}, nil
	case "min":
		return &Min{}, nil
	case "sum":
		return &Sum{}, nil
	}
	return nil, fmt.Errorf("invalid AggregateFunction type %s", function)
}

func AggregateConfigurationOfDuration(d time.Duration) *AggregateConfiguration {
	if d >= (time.Hour * 24) {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitDay,
			WindowAmount: uint64(d / (time.Hour * 24)),
		}
	} else if d >= time.Hour {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitHour,
			WindowAmount: uint64(d / time.Hour),
		}
	} else if d >= time.Minute {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitMinute,
			WindowAmount: uint64(d / time.Minute),
		}
	} else if d >= time.Second {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitSecond,
			WindowAmount: uint64(d / time.Second),
		}
	} else if d >= time.Millisecond {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitMillisecond,
			WindowAmount: uint64(d / time.Millisecond),
		}
	} else if d >= time.Microsecond {
		return &AggregateConfiguration{
			WindowUnit:   WindowUnitMicrosecond,
			WindowAmount: uint64(d / time.Microsecond),
		}
	}
	return &AggregateConfiguration{
		WindowUnit:   WindowUnitNanosecond,
		WindowAmount: uint64(d / time.Nanosecond),
	}
}

type AggregateConfiguration struct {
	WindowUnit   WindowUnit
	WindowAmount uint64
	Function     AggregateFunction
	CreateEmpty  bool
}

type ElectricityUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*ElectricityUsage
}

type ElectricityStateRecord struct {
	Time time.Time
	Name string
	Role string
	*ElectricityState
}

type GasUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*GasUsage
}

type WaterUsageRecord struct {
	Time time.Time
	Name string
	Role string
	*WaterUsage
}

type ElectricityCostRecord struct {
	Time                   time.Time
	Name                   string
	ConsumptionCosts       float32
	ConsumptionPricePerKwh float32
	ConsumptionEnergy      float32
	FeedbackCosts          float32
	FeedbackPricePerKwh    float32
	FeedbackEnergy         float32
}

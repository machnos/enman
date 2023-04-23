package persistency

import (
	"fmt"
	"strings"
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
	}
	return nil, fmt.Errorf("invalid AggregateFunction type %s", function)
}

type AggregateConfiguration struct {
	WindowUnit   WindowUnit
	WindowAmount uint64
	Function     AggregateFunction
	CreateEmpty  bool
}

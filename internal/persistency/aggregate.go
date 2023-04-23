package persistency

import (
	"fmt"
	"strings"
)

type WindowUnit uint8

const (
	Nanosecond WindowUnit = iota
	Microsecond
	Millisecond
	Second
	Minute
	Hour
	Day
	Week
	Month
	Year
)

func WindowUnitOf(unit string) (WindowUnit, error) {
	switch strings.ToLower(unit) {
	case "nanosecond":
		return Nanosecond, nil
	case "microsecond":
		return Microsecond, nil
	case "millisecond":
		return Millisecond, nil
	case "second":
		return Second, nil
	case "minute":
		return Minute, nil
	case "hour":
		return Hour, nil
	case "day":
		return Day, nil
	case "week":
		return Week, nil
	case "month":
		return Month, nil
	case "Year":
		return Year, nil
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

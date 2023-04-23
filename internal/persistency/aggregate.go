package persistency

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

type AggregateConfiguration struct {
	WindowUnit   WindowUnit
	WindowAmount uint16
	Function     AggregateFunction
	CreateEmpty  bool
}

package influx

import (
	"enman/internal/log"
	"fmt"
	"time"
)

type ComparisonOperator string

const (
	Equals             ComparisonOperator = "=="
	NotEquals          ComparisonOperator = "!="
	LessThan           ComparisonOperator = "<"
	GreaterThan        ComparisonOperator = ">"
	LessThanOrEquals   ComparisonOperator = "<="
	GreaterThanOrEqual ComparisonOperator = ">="
	EqualsRegEx        ComparisonOperator = "=~"
	NotEqualsRegEx     ComparisonOperator = "!~"
)

type AggregateFunction string

const (
	Count  AggregateFunction = "count"
	Max    AggregateFunction = "max"
	Mean   AggregateFunction = "mean"
	Median AggregateFunction = "median"
	Min    AggregateFunction = "min"
	Sum    AggregateFunction = "sum"
)

type Statement interface {
	StatementString() string
}

type QueryBuilder struct {
	querySource QuerySource
	statements  []Statement
}

func NewQueryBuilder(querySource QuerySource) *QueryBuilder {
	return &QueryBuilder{
		querySource: querySource,
		statements:  make([]Statement, 0),
	}
}

func (q *QueryBuilder) Append(statement Statement) *QueryBuilder {
	q.statements = append(q.statements, statement)
	return q
}

func (q *QueryBuilder) Build() string {
	query := q.querySource.QuerySourceString()
	for _, statement := range q.statements {
		query += fmt.Sprintf(`|> %s`, statement.StatementString())
	}
	if log.TraceEnabled() {
		log.Tracef("Building influx query: %s", query)
	}
	return query
}

type RangeStatement struct {
	start time.Time
	stop  time.Time
}

func (rs *RangeStatement) StatementString() string {
	filter := fmt.Sprintf("start: %d", rs.start.Unix())
	if !rs.stop.IsZero() {
		if len(filter) > 0 {
			filter += ", "
		}
		filter += fmt.Sprintf("stop: %d", rs.stop.Unix())
	}
	if len(filter) == 0 {
		return filter
	}
	return fmt.Sprintf("range(%s)", filter)
}

func NewRangeStatement(from time.Time, till time.Time) *RangeStatement {
	return &RangeStatement{
		start: from,
		stop:  till,
	}
}

type FilterStatement struct {
	keepOnEmpty    bool
	filterFunction *FilterFunction
}

func (fs *FilterStatement) SetKeepOnEmpty(keepOnEmpty bool) *FilterStatement {
	fs.keepOnEmpty = keepOnEmpty
	return fs
}

type FilterFunction struct {
	field        string
	comparison   ComparisonOperator
	value        any
	andFunctions []*FilterFunction
	orFunctions  []*FilterFunction
}

func (ff *FilterFunction) And(function *FilterFunction) *FilterFunction {
	ff.andFunctions = append(ff.andFunctions, function)
	return ff
}

func (ff *FilterFunction) Or(function *FilterFunction) *FilterFunction {
	ff.orFunctions = append(ff.orFunctions, function)
	return ff
}

func (ff *FilterFunction) string(recordVariable string) string {
	var value any = nil
	stringValue, ok := ff.value.(string)
	if ok && (ff.comparison != EqualsRegEx && ff.comparison != NotEqualsRegEx) {
		value = fmt.Sprintf("\"%s\"", stringValue)
	} else {
		value = ff.value
	}
	function := fmt.Sprintf("%s[\"%s\"] %s %v", recordVariable, ff.field, ff.comparison, value)
	if len(ff.andFunctions) == 0 && len(ff.orFunctions) == 0 {
		return function
	}
	function = "(" + function
	for _, andFunction := range ff.andFunctions {
		function = function + " and " + andFunction.string(recordVariable)
	}
	for _, orFunction := range ff.orFunctions {
		function = function + " or " + orFunction.string(recordVariable)
	}
	function = function + ")"
	return function
}

func NewFilterFunction(field string, comparison ComparisonOperator, value any) *FilterFunction {
	return &FilterFunction{
		field:        field,
		comparison:   comparison,
		value:        value,
		andFunctions: make([]*FilterFunction, 0),
		orFunctions:  make([]*FilterFunction, 0),
	}
}

func (fs *FilterStatement) StatementString() string {
	onEmpty := "drop"
	if fs.keepOnEmpty {
		onEmpty = "keep"
	}
	return fmt.Sprintf("filter(fn: (r) => %s, onEmpty: \"%s\")", fs.filterFunction.string("r"), onEmpty)
}

func NewFilterStatement(filterFunction *FilterFunction) *FilterStatement {
	return &FilterStatement{
		filterFunction: filterFunction,
	}
}

type AggregateWindowStatement struct {
	every       string
	function    AggregateFunction
	createEmpty bool
	offset      string
}

func (aws *AggregateWindowStatement) SetOffset(offset string) *AggregateWindowStatement {
	aws.offset = offset
	return aws
}

func (aws *AggregateWindowStatement) StatementString() string {
	agg := fmt.Sprintf("every: %s, fn: %s, createEmpty: %t", aws.every, aws.function, aws.createEmpty)
	if aws.offset != "" {
		agg += ", offset: " + aws.offset
	}
	return "aggregateWindow(" + agg + ")"
}

func NewAggregateWindowStatement(every string, function AggregateFunction, createEmpty bool) *AggregateWindowStatement {
	return &AggregateWindowStatement{
		every:       every,
		function:    function,
		createEmpty: createEmpty,
	}
}

type PivotStatement struct {
	columnKey   string
	rowKey      string
	valueColumn string
}

func (ps *PivotStatement) StatementString() string {
	return fmt.Sprintf("pivot(columnKey:[\"%s\"], rowKey: [\"%s\"], valueColumn: \"%s\")", ps.columnKey, ps.rowKey, ps.valueColumn)
}

func NewPivotStatement(columnKey string, rowKey string, valueColumn string) *PivotStatement {
	return &PivotStatement{
		columnKey:   columnKey,
		rowKey:      rowKey,
		valueColumn: valueColumn,
	}
}

type SortStatement struct {
	columns []string
	desc    bool
}

func (ss *SortStatement) SetAscending() *SortStatement {
	ss.desc = false
	return ss
}

func (ss *SortStatement) SetDescending() *SortStatement {
	ss.desc = true
	return ss
}

func (ss *SortStatement) StatementString() string {
	columns := ""
	for _, column := range ss.columns {
		if columns != "" {
			columns += ", "
		}
		columns += "\"" + column + "\""
	}
	return fmt.Sprintf("sort(columns:[%s], desc: %t)", columns, ss.desc)
}

func NewSortStatement(columns ...string) *SortStatement {
	if len(columns) == 0 {
		columns = []string{"_default"}
	}
	return &SortStatement{
		columns: columns,
	}
}

type LimitStatement struct {
	numberOfRows uint16
	offset       uint16
}

func (ls *LimitStatement) SetOffset(offset uint16) *LimitStatement {
	ls.offset = offset
	return ls
}

func (ls *LimitStatement) StatementString() string {
	limit := fmt.Sprintf("n: %d", ls.numberOfRows)
	if ls.offset > 0 {
		limit += fmt.Sprintf(", offset: %d", ls.offset)
	}
	return "limit(" + limit + ")"
}

func NewLimitStatement(numberOfRows uint16) *LimitStatement {
	return &LimitStatement{
		numberOfRows: numberOfRows,
	}
}

type TimeShiftStatement struct {
	duration string
}

func (tst *TimeShiftStatement) StatementString() string {
	duration := fmt.Sprintf("duration: %s", tst.duration)
	return "timeShift(" + duration + ")"
}

func NewTimeShiftStatement(duration string) *TimeShiftStatement {
	return &TimeShiftStatement{
		duration: duration,
	}
}

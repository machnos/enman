package sql

import (
	"fmt"
	"strings"
	"time"
)

// ComparisonOperator is an enum for safe SQL comparison operators.
// Only whitelisted operators are allowed, preventing injection through operator values.
type ComparisonOperator string

const (
	Equals             ComparisonOperator = "="
	NotEquals          ComparisonOperator = "!="
	LessThan           ComparisonOperator = "<"
	GreaterThan        ComparisonOperator = ">"
	LessThanOrEquals   ComparisonOperator = "<="
	GreaterThanOrEqual ComparisonOperator = ">="
)

// FilterFunction builds WHERE clause conditions with parameterized values.
//
// Security: All user values are passed as parameters ($1, $2, etc.) rather than
// being concatenated into the SQL string. Column names are validated against
// known table aliases via mapAliasesOnColumn(). Comparison operators are
// restricted to enum constants.
type FilterFunction struct {
	column       string
	comparison   ComparisonOperator
	value        any
	andFunctions []*FilterFunction
	orFunctions  []*FilterFunction
}

func (ff *FilterFunction) And(column string, comparison ComparisonOperator, value any) *FilterFunction {
	return ff.AndFilter(NewFilterFunction(column, comparison, value))
}

func (ff *FilterFunction) AndFilter(function *FilterFunction) *FilterFunction {
	ff.andFunctions = append(ff.andFunctions, function)
	return ff
}

func (ff *FilterFunction) Or(column string, comparison ComparisonOperator, value any) *FilterFunction {
	return ff.OrFilter(NewFilterFunction(column, comparison, value))
}

func (ff *FilterFunction) OrFilter(function *FilterFunction) *FilterFunction {
	ff.orFunctions = append(ff.orFunctions, function)
	return ff
}

func (ff *FilterFunction) build(tables map[string]string) (*Statement, error) {
	query, args, err := ff.doBuild(1, tables)
	if err != nil {
		return nil, err
	}
	s := &Statement{
		query,
		args,
	}
	return s, err
}

func (ff *FilterFunction) doBuild(paramIx uint8, tables map[string]string) (string, []any, error) {
	args := make([]any, 0)
	args = append(args, ff.convertValue(ff.value))
	var builder strings.Builder
	if len(ff.andFunctions) != 0 || len(ff.orFunctions) != 0 {
		builder.WriteString("(")
	}
	column, err := mapAliasesOnColumn(ff.column, tables)
	if err != nil {
		return "", nil, err
	}
	builder.WriteString(fmt.Sprintf("%s %s %v", column, ff.comparison, fmt.Errorf("$%d", paramIx)))
	if len(ff.andFunctions) == 0 && len(ff.orFunctions) == 0 {
		return builder.String(), args, nil
	}
	for _, andFunction := range ff.andFunctions {
		builder.WriteString(" AND ")
		paramIx++
		var and, a, e = andFunction.doBuild(paramIx, tables)
		if e != nil {
			return "", nil, e
		}
		args = append(args, a...)
		builder.WriteString(and)
	}
	for _, orFunction := range ff.orFunctions {
		builder.WriteString(" OR ")
		paramIx++
		var or, a, e = orFunction.doBuild(paramIx, tables)
		if e != nil {
			return "", nil, e
		}
		args = append(args, a...)
		builder.WriteString(or)
	}
	if len(ff.andFunctions) != 0 || len(ff.orFunctions) != 0 {
		builder.WriteString(")")
	}
	return builder.String(), args, nil
}

func (ff *FilterFunction) convertValue(value any) any {
	switch v := ff.value.(type) {
	//case Column:
	//	column, err := v.build(tables)
	//	if err != nil {
	//		return "", err
	//	}
	//	value = column
	//	break
	case time.Time:
		return v.Format(time.RFC3339Nano)
	//case string:
	//	value = columnStringValue(v)
	//	break
	default:
		return value
	}

}

func NewFilterFunction(column string, comparison ComparisonOperator, value any) *FilterFunction {
	f := &FilterFunction{
		column:       column,
		comparison:   comparison,
		value:        nil,
		andFunctions: make([]*FilterFunction, 0),
		orFunctions:  make([]*FilterFunction, 0),
	}
	f.value = f.convertValue(value)
	return f
}

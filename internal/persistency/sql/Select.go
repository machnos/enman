package sql

import (
	"fmt"
	"strings"
)

// Select builds parameterized SQL SELECT statements safely.
//
// Security: This query builder prevents SQL injection through:
//  1. Table names are hardcoded constants from the timescale package (e.g., "electricity_states")
//     and passed to NewSelect(). Users cannot control table names.
//  2. Column names come from Column structs which are instantiated with compile-time string literals.
//     mapAliasesOnColumn() validates column references against known table aliases.
//  3. All user values are parameterized using $1, $2, etc. (see FilterFunction.doBuild())
//  4. Comparison operators are enum types (ComparisonOperator) with whitelisted constants.
//
// This design ensures no string concatenation of user input into SQL queries.
type Select struct {
	table          string
	tableAliases   map[string]string
	columns        []*Column
	joins          []*Join
	filterFunction *FilterFunction
	groupBy        []*Column
	orderBy        []*Column
	distinct       bool
	orderAscending bool
}

// NewSelect creates a new SELECT builder for the given table.
// Note: table parameter must be a hardcoded table name constant (not user input).
func NewSelect(table string) *Select {
	var s = &Select{
		tableAliases: make(map[string]string),
	}
	s.table = table
	s.tableAliases[table] = "t0"
	return s
}

func (s *Select) Distinct() *Select {
	s.distinct = true
	return s
}

func (s *Select) WithJoin(join *Join) *Select {
	s.tableAliases[join.table] = fmt.Sprintf("t%d", len(s.tableAliases))
	s.joins = append(s.joins, join)
	return s
}

func (s *Select) WithColumns(columns ...*Column) *Select {
	s.columns = append(s.columns, columns...)
	return s
}

func (s *Select) WithFilter(function *FilterFunction) *Select {
	s.filterFunction = function
	return s
}

func (s *Select) GroupBy(column ...*Column) *Select {
	s.groupBy = column
	return s
}

func (s *Select) OrderAscending(column ...*Column) *Select {
	s.orderBy = column
	s.orderAscending = true
	return s
}

func (s *Select) OrderDescending(column ...*Column) *Select {
	s.orderBy = column
	s.orderAscending = false
	return s
}

func (s *Select) Build() (*Statement, error) {
	result := &Statement{}
	var builder strings.Builder
	builder.WriteString("SELECT ")
	if s.distinct {
		builder.WriteString("DISTINCT ")
	}
	if len(s.columns) == 0 {
		builder.WriteString("*")
	} else {
		for ix, column := range s.columns {
			if ix > 0 {
				builder.WriteString(", ")
			}
			columnStatement, err := column.build(s.tableAliases)
			if err != nil {
				return nil, err
			}
			builder.WriteString(columnStatement)
		}
	}
	builder.WriteString(" FROM ")
	builder.WriteString(s.table)
	if len(s.joins) != 0 {
		builder.WriteString(fmt.Sprintf(" AS %s ", s.tableAliases[s.table]))
		for _, join := range s.joins {
			j, err := join.Build(s.table, s.tableAliases)
			if err != nil {
				return nil, err
			}
			builder.WriteString(j)
		}
	}
	if s.filterFunction != nil {
		filter, err := s.filterFunction.build(s.tableAliases)
		if err != nil {
			return nil, err
		}
		builder.WriteString(fmt.Sprintf(" WHERE %s", filter.Query))
		result.Args = append(result.Args, filter.Args...)
	}
	if s.groupBy != nil {
		builder.WriteString(" GROUP BY (")
		for ix, gbColumn := range s.groupBy {
			if ix > 0 {
				builder.WriteString(", ")
			}
			column := gbColumn.as
			if column == "" {
				c, err := mapAliasesOnColumn(gbColumn.name, s.tableAliases)
				if err != nil {
					return nil, err
				}
				column = c
			}
			builder.WriteString(column)
		}
		builder.WriteString(")")
	}
	if s.orderBy != nil {
		builder.WriteString(" ORDER BY (")
		for ix, gbColumn := range s.orderBy {
			if ix > 0 {
				builder.WriteString(", ")
			}
			column := gbColumn.as
			if column == "" {
				c, err := mapAliasesOnColumn(gbColumn.name, s.tableAliases)
				if err != nil {
					return nil, err
				}
				column = c
			}
			builder.WriteString(column)
		}
		builder.WriteString(")")
		if s.orderAscending {
			builder.WriteString(" ASC")
		} else {
			builder.WriteString(" DESC")
		}
	}
	builder.WriteString(";")
	result.Query = builder.String()
	return result, nil
}

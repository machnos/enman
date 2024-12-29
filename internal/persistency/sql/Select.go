package sql

import (
	"fmt"
	"strings"
)

type Select struct {
	table          string
	tableAliases   map[string]string
	columns        []*Column
	joins          []*Join
	filterFunction *FilterFunction
	groupBy        []*Column
	orderBy        []*Column
	orderAscending bool
}

func NewSelect(table string) *Select {
	var s = &Select{
		tableAliases: make(map[string]string),
	}
	s.table = table
	s.tableAliases[table] = "t0"
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

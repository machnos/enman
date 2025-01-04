package sql

import (
	"fmt"
	"strings"
)

func mapAliasesOnColumn(column string, tables map[string]string) (string, error) {
	parts := strings.Split(column, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("too many parts in column name: %s", column)
	}
	if len(parts) == 1 {
		if len(tables) != 1 {
			return "", fmt.Errorf("colum %s should be in the format <tablename>.%s", column, column)
		}
		return column, nil
	}
	value, exists := tables[parts[0]]
	if !exists {
		return "", fmt.Errorf("unknown table reference: %s", column)
	}
	return fmt.Sprintf("%s.%s", value, parts[1]), nil
}

type Column struct {
	name           string
	as             string
	columnFunction func(string) string
}

func NewColumn(name string, as string, columnFunction func(string) string) *Column {
	return &Column{
		name:           name,
		as:             as,
		columnFunction: columnFunction,
	}
}

func NewColumns(names ...string) []*Column {
	columns := make([]*Column, 0)
	for _, name := range names {
		columns = append(columns, NewColumn(name, "", nil))
	}
	return columns
}

func NewColumnsWithFunctions(columnFunctions []func(string) string, names ...string) []*Column {
	columns := make([]*Column, 0)
	for _, columnFunction := range columnFunctions {
		for _, name := range names {
			columns = append(columns, NewColumn(name, "", columnFunction))
		}
	}
	return columns
}

func NewColumnWithName(name string) *Column {
	return NewColumnWithNameAs(name, "")
}

func NewColumnWithNameAs(name string, as string) *Column {
	return NewColumn(name, as, nil)
}

func NewCountColumn(name string) *Column {
	return NewCountColumnAs(name, "")
}

func NewCountColumnAs(name string, as string) *Column {
	return NewColumn(name, as, func(col string) string {
		return fmt.Sprintf("COUNT(%s)", col)
	})
}

func NewCountDistinctColumn(name string) *Column {
	return NewCountDistinctColumnAs(name, "")
}

func NewCountDistinctColumnAs(name string, as string) *Column {
	return NewColumn(name, as, func(col string) string {
		return fmt.Sprintf("COUNT(DISTINCT(%s))", col)
	})
}

func (c *Column) build(tables map[string]string) (string, error) {
	column, err := mapAliasesOnColumn(c.name, tables)
	if err != nil {
		return "", err
	}
	if c.columnFunction != nil {
		column = c.columnFunction(column)
	}
	if c.as == "" {
		return column, nil
	} else {
		return column + " AS " + c.as, nil
	}
}

package sql

import (
	"fmt"
	"slices"
	"strings"
)

type TableDefinition struct {
	Name        string
	Columns     []*ColumnDefinition
	PrimaryKey  []string
	ForeignKeys []*ForeignKey
}

type ColumnDefinition struct {
	Name     string
	SqlType  string
	Nullable bool
}

type ForeignKey struct {
	SourceColumns []string
	TargetTable   string
	TargetColumns []string
}

func (t *TableDefinition) ColumnNames() []string {
	var result []string
	for _, column := range t.Columns {
		result = append(result, column.Name)
	}
	return result
}

func (t *TableDefinition) TablePrefixedColumnNames() []string {
	var result []string
	for _, column := range t.Columns {
		result = append(result, fmt.Sprintf("%s.%s", t.Name, column.Name))
	}
	return result
}

func (t *TableDefinition) TablePrefixedColumn(column string) string {
	return fmt.Sprintf("%s.%s", t.Name, column)
}

func (t *TableDefinition) CreateStatement() string {
	var builder strings.Builder
	builder.WriteString("CREATE TABLE ")
	builder.WriteString(t.Name)
	builder.WriteString(" (")
	for ix, column := range t.Columns {
		if ix > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%s %s", column.Name, column.SqlType))
		if !column.Nullable {
			builder.WriteString(" NOT NULL")
		}
	}
	if len(t.PrimaryKey) > 0 {
		builder.WriteString(", PRIMARY KEY (")
		for ix, pkColumn := range t.PrimaryKey {
			if ix > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(pkColumn)
		}
		builder.WriteString(")")
	}
	for _, fk := range t.ForeignKeys {
		builder.WriteString(fmt.Sprintf(", FOREIGN KEY (%s) REFERENCES %s (%s)", strings.Join(fk.SourceColumns, ","), fk.TargetTable, strings.Join(fk.TargetColumns, ",")))
	}
	builder.WriteString(")")
	return builder.String()
}

func (t *TableDefinition) InsertStatement() string {
	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	builder.WriteString(t.Name)
	builder.WriteString(" (")
	for ix, column := range t.Columns {
		if ix > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(column.Name)
	}
	builder.WriteString(") VALUES (")
	for ix, _ := range t.Columns {
		if ix > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("$%d", ix+1))
	}
	builder.WriteString(");")
	return builder.String()
}

func (t *TableDefinition) UpsertStatement(dialect Dialect) string {
	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	builder.WriteString(t.Name)
	builder.WriteString(" (")
	for ix, column := range t.Columns {
		if ix > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(column.Name)
	}
	builder.WriteString(") VALUES (")
	for ix, _ := range t.Columns {
		if ix > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("$%d", ix+1))
	}
	builder.WriteString(")")
	switch dialect {
	case Timescale:
		builder.WriteString(fmt.Sprintf(" ON CONFLICT ("))
		for ix, pkColumn := range t.PrimaryKey {
			if ix > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(pkColumn)
		}
		builder.WriteString(")")
		added := false
		for _, column := range t.Columns {
			if slices.Contains(t.PrimaryKey, column.Name) {
				continue
			}
			if added {
				builder.WriteString(",")
			} else {
				builder.WriteString(" DO UPDATE SET")
			}
			builder.WriteString(fmt.Sprintf(" %s = EXCLUDED.%s", column.Name, column.Name))
			added = true
		}
		if !added {
			builder.WriteString(" DO NOTHING")
		}
		break
	}
	builder.WriteString(";")
	return builder.String()
}

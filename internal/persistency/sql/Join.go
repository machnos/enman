package sql

import "fmt"

type JoinType string

const (
	Inner JoinType = "INNER JOIN"
)

type Join struct {
	table    string
	joinType JoinType
	column1  string
	column2  string
}

func NewJoin(table string, joinType JoinType, column1 string, column2 string) *Join {
	return &Join{
		table,
		joinType,
		column1,
		column2,
	}
}

func (j *Join) Build(table string, tables map[string]string) (string, error) {
	targetTableAlias, _ := tables[j.table]
	column1, err := mapAliasesOnColumn(j.column1, tables)
	if err != nil {
		return "", err
	}
	column2, err := mapAliasesOnColumn(j.column2, tables)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s AS %s ON %s = %s", j.joinType, j.table, targetTableAlias, column1, column2), nil
}

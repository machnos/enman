package sql

import "testing"

func TestSelectCount(t *testing.T) {
	var expected = "SELECT COUNT(column1) FROM table1;"
	statement, err := NewSelect("table1").WithColumns(NewCountColumn("column1")).Build()
	if err != nil {
		t.Error(err)
	}
	if statement.Query != expected {
		t.Errorf("query expected = %s, got %s", expected, statement.Query)
	}
}

func TestSelectDistinct(t *testing.T) {
	var expected = "SELECT DISTINCT column1 FROM table1;"
	statement, err := NewSelect("table1").Distinct().WithColumns(NewColumnWithName("column1")).Build()
	if err != nil {
		t.Error(err)
	}
	if statement.Query != expected {
		t.Errorf("query expected = %s, got %s", expected, statement.Query)
	}
}

func TestSelectCountDistinct(t *testing.T) {
	var expected = "SELECT COUNT(DISTINCT(column1)) FROM table1;"
	statement, err := NewSelect("table1").WithColumns(NewCountDistinctColumn("column1")).Build()
	if err != nil {
		t.Error(err)
	}
	if statement.Query != expected {
		t.Errorf("query expected = %s, got %s", expected, statement.Query)
	}
}

func TestSelectWhere(t *testing.T) {
	var expected = "SELECT t0.column1, t0.column2, t1.column2 FROM table0 AS t0 INNER JOIN table1 AS t1 ON t0.column1 = t1.column1;"
	statement, err := NewSelect("table0").
		WithJoin(NewJoin("table1", Inner, "table0.column1", "table1.column1")).
		WithColumns(NewColumns("table0.column1", "table0.column2", "table1.column2")...).
		Build()
	if err != nil {
		t.Error(err)
	}
	if statement.Query != expected {
		t.Errorf("query expected = %s, got %s", expected, statement.Query)
	}
}

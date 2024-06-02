package arithmetic

import (
	"fmt"
	"testing"
)

func TestSimpleNumber(t *testing.T) {
	expectedValue := 3.14
	expectedValueAsString := fmt.Sprintf("%f", expectedValue)
	value, err := ParseCalculation(expectedValueAsString, nil)
	if err != nil {
		t.Errorf("unable to parse simple number: %v", err)
	}
	if expectedValue != value {
		t.Errorf("unable to parse simple number, expected = %f, got %f", expectedValue, value)
	}
}

func TestAddition(t *testing.T) {
	leftValue := 1.5
	middleValue := float64(3)
	rightValue := float64(2)
	expectedValue := leftValue + middleValue + rightValue

	value, err := ParseCalculation(fmt.Sprintf("%f + %f  + %f ", leftValue, middleValue, rightValue), nil)
	if err != nil {
		t.Errorf("unable to parse addition: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid addition, expected = %f, got %f", expectedValue, value)
	}
}

func TestSubtraction(t *testing.T) {
	leftValue := 1.5
	middleValue := float64(3)
	rightValue := float64(2)
	expectedValue := leftValue - middleValue - rightValue

	value, err := ParseCalculation(fmt.Sprintf("%f - %f- %f ", leftValue, middleValue, rightValue), nil)
	if err != nil {
		t.Errorf("unable to parse subtraction: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid subtraction, expected = %f, got %f", expectedValue, value)
	}
}

func TestDivision(t *testing.T) {
	leftValue := float64(9)
	middleValue := float64(3)
	rightValue := float64(2)
	expectedValue := leftValue / middleValue / rightValue

	value, err := ParseCalculation(fmt.Sprintf("%f / %f/ %f ", leftValue, middleValue, rightValue), nil)
	if err != nil {
		t.Errorf("unable to parse division: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid division, expected = %f, got %f", expectedValue, value)
	}
}

func TestMultiplicity(t *testing.T) {
	leftValue := 1.5
	middleValue := float64(3)
	rightValue := float64(2)
	expectedValue := leftValue * middleValue * rightValue

	value, err := ParseCalculation(fmt.Sprintf("%f * %f * %f ", leftValue, middleValue, rightValue), nil)
	if err != nil {
		t.Errorf("unable to parse multiplicity: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid multiplicity, expected = %f, got %f", expectedValue, value)
	}
}

func TestParenthesis(t *testing.T) {
	expectedValue := (3 * (6 / 2)) / (1.5 * 2)

	value, err := ParseCalculation(fmt.Sprintf("(3 * 3) / (1.5 * 2)"), nil)
	if err != nil {
		t.Errorf("unable to parse parenthesis: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid parenthesis calculation, expected = %f, got %f", expectedValue, value)
	}
}

func TestOrder(t *testing.T) {
	expectedValue := 1 - 2 + 9/1.5*2.0

	value, err := ParseCalculation(fmt.Sprintf("1 - 2 + 9 / 1.5 * 2.0"), nil)
	if err != nil {
		t.Errorf("unable to parse expression: %v", err)
	}
	if expectedValue != value {
		t.Errorf("invalid order calculation, expected = %f, got %f", expectedValue, value)
	}
}

func TestVariable(t *testing.T) {
	expectedValue := 3.14
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = expectedValue
	value, err := ParseCalculation(fmt.Sprintf("${%s}", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if expectedValue != value {
		t.Errorf("unable to parse simple number, expected = %f, got %f", expectedValue, value)
	}
}

func TestLessThanOrEqual(t *testing.T) {
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = 3.13
	value, err := ParseExpression(fmt.Sprintf("${%s} <= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with <= comparison")
	}

	variables[variableName] = 3.14
	value, err = ParseExpression(fmt.Sprintf("${%s} <= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with <= comparison")
	}

	variables[variableName] = 3.15
	value, err = ParseExpression(fmt.Sprintf("${%s} <= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with <= comparison")
	}
}

func TestLessThan(t *testing.T) {
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = 3.13
	value, err := ParseExpression(fmt.Sprintf("${%s} < 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with < comparison")
	}

	variables[variableName] = 3.14
	value, err = ParseExpression(fmt.Sprintf("${%s} < 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with < comparison")
	}

	variables[variableName] = 3.15
	value, err = ParseExpression(fmt.Sprintf("${%s} < 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with < comparison")
	}
}

func TestEqual(t *testing.T) {
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = 3.13
	value, err := ParseExpression(fmt.Sprintf("${%s} == 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with == comparison")
	}

	variables[variableName] = 3.14
	value, err = ParseExpression(fmt.Sprintf("${%s} == 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with == comparison")
	}

	variables[variableName] = 3.15
	value, err = ParseExpression(fmt.Sprintf("${%s} == 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with == comparison")
	}
}

func TestGreaterThan(t *testing.T) {
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = 3.13
	value, err := ParseExpression(fmt.Sprintf("${%s} > 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with > comparison")
	}

	variables[variableName] = 3.14
	value, err = ParseExpression(fmt.Sprintf("${%s} > 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with > comparison")
	}

	variables[variableName] = 3.15
	value, err = ParseExpression(fmt.Sprintf("${%s} > 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with > comparison")
	}
}

func TestGreaterThanOrEqual(t *testing.T) {
	variableName := "base"
	variables := make(map[string]float64)
	variables[variableName] = 3.13
	value, err := ParseExpression(fmt.Sprintf("${%s} >= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if false != value {
		t.Errorf("Failed to validate numbers with >= comparison")
	}

	variables[variableName] = 3.14
	value, err = ParseExpression(fmt.Sprintf("${%s} >= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with >= comparison")
	}

	variables[variableName] = 3.15
	value, err = ParseExpression(fmt.Sprintf("${%s} >= 3.14", variableName), variables)
	if err != nil {
		t.Errorf("unable to parse variable: %v", err)
	}
	if true != value {
		t.Errorf("Failed to validate numbers with >= comparison")
	}
}

package arithmetic

import (
	"fmt"
	"strconv"
)

type number struct {
	value float64
}

type operation struct {
	priority uint8
	function func(*number, *number) *number
}

func ParseExpression(expression string, variables map[string]float64) (float64, error) {
	l := newLexer(expression)
	nr, err := readNumber(l, variables)
	if err != nil {
		return 0, err
	}
	// We've found the number, now check for the operation
	op, err := readOperation(l)
	if err != nil {
		return 0, err
	}
	if op == nil {
		return nr.value, nil
	}
	formulas := make([]any, 0)
	formulas = append(formulas, nr)
	formulas = append(formulas, op)
	if !l.hasNext() {
		return 0, fmt.Errorf("unable to parse expression: %s", expression)
	}
	for l.hasNext() {
		nr, err = readNumber(l, variables)
		if err != nil {
			return 0, err
		}
		formulas = append(formulas, nr)
		if l.hasNext() {
			op, err = readOperation(l)
			if err != nil {
				return 0, err
			}
			if op == nil {
				return 0, fmt.Errorf("unable to parse expression: %s", expression)
			}
			formulas = append(formulas, op)
		}
	}
	currentPriority := uint8(2)
	for currentPriority > 0 && len(formulas) > 1 {
		for ix := 0; ix < len(formulas); {
			o, ok := formulas[ix].(*operation)
			if !ok || o.priority != currentPriority {
				ix++
				continue
			}
			if o.priority == currentPriority {
				leftNr := formulas[ix-1].(*number)
				rightNr := formulas[ix+1].(*number)

				newFormulas := make([]any, 0)
				if ix-1 > 0 {
					newFormulas = append(newFormulas, formulas[0:ix-1]...)
				}
				newFormulas = append(newFormulas, o.function(leftNr, rightNr))
				if ix+2 < len(formulas) {
					newFormulas = append(newFormulas, formulas[ix+2:]...)
				}
				formulas = newFormulas
			}
		}
		currentPriority--
	}
	return formulas[0].(*number).value, nil
}

func readNumber(l *lexer, variables map[string]float64) (*number, error) {
	if !l.hasNext() {
		return nil, fmt.Errorf("empty lexer")
	}
	c := l.next()
	if c == '(' {
		// Open parenthesis -> read until close parenthesis
		nrOfOpenParenthesis := 1
		subExpression := ""
		for l.hasNext() {
			c = l.next()
			if c == '(' {
				nrOfOpenParenthesis++
				continue
			}
			if c == ')' {
				nrOfOpenParenthesis--
				if nrOfOpenParenthesis == 0 {
					value, err := ParseExpression(subExpression, variables)
					if err != nil {
						return nil, err
					}
					return &number{
						value: value,
					}, nil
				}
			}
			subExpression += string(c)
		}
		return nil, fmt.Errorf("no closing parenthesis found: %s", subExpression)
	} else if c == '$' {
		if !l.hasNext() {
			return nil, fmt.Errorf("starting a variable ($) at the end of an expression")
		}
		c = l.next()
		if c != '{' {
			return nil, fmt.Errorf("variable should be in the form of ${name}")
		}
		variableName := ""
		for l.hasNext() {
			c = l.next()
			if c == '}' {
				break
			}
			variableName += string(c)
			if !l.hasNext() {
				return nil, fmt.Errorf("no closing bracket found to end variable: %s", variableName)
			}
		}
		value, ok := variables[variableName]
		if !ok {
			return nil, fmt.Errorf("no value for variable %s", variableName)
		}
		return &number{
			value: value,
		}, nil
	}
	stringNumber := string(c)
	for l.hasNext() {
		c = l.next()
		if c == '+' || c == '-' || c == '*' || c == '/' {
			l.rewind(1)
			break
		} else {
			stringNumber += string(c)
		}
	}
	value, err := strconv.ParseFloat(stringNumber, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse numeric value : %s", stringNumber)
	}
	number := &number{
		value: value,
	}
	return number, nil

}

func readOperation(l *lexer) (*operation, error) {
	if !l.hasNext() {
		return nil, nil
	}
	c := l.next()
	if c == '+' {
		return &operation{
			priority: 1,
			function: func(first *number, second *number) *number {
				return &number{
					value: first.value + second.value,
				}
			},
		}, nil
	} else if c == '-' {
		return &operation{
			priority: 1,
			function: func(first *number, second *number) *number {
				return &number{
					value: first.value - second.value,
				}
			},
		}, nil
	} else if c == '*' {
		return &operation{
			priority: 2,
			function: func(first *number, second *number) *number {
				return &number{
					value: first.value * second.value,
				}
			},
		}, nil
	} else if c == '/' {
		return &operation{
			priority: 2,
			function: func(first *number, second *number) *number {
				return &number{
					value: first.value / second.value,
				}
			},
		}, nil
	}
	return nil, fmt.Errorf("unknown operation: %s", string(c))
}

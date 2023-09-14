package arithmetic

type lexer struct {
	runes    []rune
	position int
}

func newLexer(expression string) *lexer {
	runes := make([]rune, 0)
	for _, character := range expression {
		if character == ' ' {
			continue
		}
		runes = append(runes, character)
	}
	return &lexer{
		runes: runes,
	}
}

func (l *lexer) next() rune {
	character := l.runes[l.position]
	l.position += 1
	return character
}

func (l *lexer) rewind(amount int) {
	l.position -= amount
}

func (l *lexer) hasNext() bool {
	return l.position < len(l.runes)
}

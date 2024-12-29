package sql

type Statement struct {
	Query string
	Args  []any
}

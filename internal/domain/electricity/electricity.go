package electricity

import (
	gpv "github.com/go-playground/validator/v10"
)

var validator = gpv.New()

const (
	// MinPhases The minimum number of phases a grid must have
	MinPhases uint8 = 1
	// MaxPhases The maximum number of phases a grid may have
	MaxPhases uint8 = 3
)

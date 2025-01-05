package gas

type Usage struct {
	gasConsumed float64
}

type usage struct {
	GasConsumed float64 `validate:"gte=0"`
}

func NewUsage() *Usage {
	return &Usage{}
}

func (u *Usage) GasConsumed() float64 {
	return u.gasConsumed
}

func (u *Usage) SetGasConsumed(gasConsumed float64) {
	u.gasConsumed = gasConsumed
}

func (u *Usage) SetValues(other *Usage) {
	u.gasConsumed = other.gasConsumed
}

func (u *Usage) rawValues() usage {
	return usage{u.gasConsumed}
}

func (u *Usage) Valid() (bool, error) {
	rawValues := u.rawValues()
	err := validator.Struct(rawValues)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (u *Usage) IsZero() bool {
	if u.gasConsumed != 0 {
		return false
	}
	return true
}

package water

type Usage struct {
	waterConsumed float64
}

type usage struct {
	WaterConsumed float64 `validate:"gte=0"`
}

func NewUsage() *Usage {
	return &Usage{}
}

func (u *Usage) WaterConsumed() float64 {
	return u.waterConsumed
}

func (u *Usage) SetWaterConsumed(waterConsumed float64) {
	u.waterConsumed = waterConsumed
}

func (u *Usage) SetValues(other *Usage) {
	u.waterConsumed = other.waterConsumed
}

func (u *Usage) rawValues() usage {
	return usage{u.waterConsumed}
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
	if u.waterConsumed != 0 {
		return false
	}
	return true
}

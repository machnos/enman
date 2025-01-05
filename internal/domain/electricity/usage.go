package electricity

type Usage struct {
	energyConsumed      [MaxPhases]float64
	totalEnergyConsumed float64
	energyProvided      [MaxPhases]float64
	totalEnergyProvided float64
}

type usage struct {
	EnergyConsumed      [MaxPhases]float64 `validate:"dive,gte=0"`
	TotalEnergyConsumed float64            `validate:"gte=0"`
	EnergyProvided      [MaxPhases]float64 `validate:"dive,gte=0"`
	TotalEnergyProvided float64            `validate:"gte=0"`
}

func NewUsage() *Usage {
	return &Usage{
		energyConsumed: [3]float64(make([]float64, MaxPhases)),
		energyProvided: [3]float64(make([]float64, MaxPhases)),
	}
}

func (u *Usage) EnergyConsumed(lineIx uint8) float64 {
	return u.energyConsumed[lineIx]
}

func (u *Usage) SetEnergyConsumed(lineIx uint8, energyConsumed float64) *Usage {
	u.energyConsumed[lineIx] = energyConsumed
	return u
}

func (u *Usage) TotalEnergyConsumed() float64 {
	if u.totalEnergyConsumed != 0 {
		return u.totalEnergyConsumed
	}
	totalEnergyConsumed := float64(0)
	for i := 0; i < len(u.energyConsumed); i++ {
		totalEnergyConsumed += u.energyConsumed[i]
	}
	return totalEnergyConsumed
}

func (u *Usage) SetTotalEnergyConsumed(totalEnergyConsumed float64) *Usage {
	u.totalEnergyConsumed = totalEnergyConsumed
	return u
}

func (u *Usage) EnergyProvided(lineIx uint8) float64 {
	return u.energyProvided[lineIx]
}

func (u *Usage) SetEnergyProvided(lineIx uint8, energyProvided float64) *Usage {
	u.energyProvided[lineIx] = energyProvided
	return u
}

func (u *Usage) TotalEnergyProvided() float64 {
	if u.totalEnergyProvided != 0 {
		return u.totalEnergyProvided
	}
	totalEnergyProvided := float64(0)
	for i := 0; i < len(u.energyProvided); i++ {
		totalEnergyProvided += u.energyProvided[i]
	}
	return totalEnergyProvided
}

func (u *Usage) SetTotalEnergyProvided(totalEnergyProvided float64) *Usage {
	u.totalEnergyProvided = totalEnergyProvided
	return u
}

func (u *Usage) IsZero() bool {
	if u.totalEnergyConsumed != 0 || u.totalEnergyProvided != 0 {
		return false
	}
	for _, value := range u.energyConsumed {
		if value != 0 {
			return false
		}
	}
	for _, value := range u.energyProvided {
		if value != 0 {
			return false
		}
	}
	return true
}

func (u *Usage) SetValues(other *Usage) {
	u.energyConsumed = other.energyConsumed
	u.energyProvided = other.energyProvided
	u.totalEnergyConsumed = other.totalEnergyConsumed
	u.totalEnergyProvided = other.totalEnergyProvided
}

func (u *Usage) rawValues() usage {
	return usage{u.energyConsumed,
		u.TotalEnergyConsumed(),
		u.energyProvided,
		u.TotalEnergyProvided()}
}

func (u *Usage) Valid() (bool, error) {
	err := validator.Struct(u.rawValues())
	if err != nil {
		return false, err
	}
	return true, nil
}

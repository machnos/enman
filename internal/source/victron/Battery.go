package victron

type Battery struct {
	nominalVoltage float32
}

func (b *Battery) SoC() uint8 {
	return 0
}

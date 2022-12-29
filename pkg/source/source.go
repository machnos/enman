package source

type Grid interface {
	Voltage() int16
	Phases() int8
	MaxCurrentPerPhase() float32
	SetPoint(watts int16) error
}

type Battery interface {
	SetPoint(watts int16) error
}

type Pv interface {
}

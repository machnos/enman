package events

import "time"

// ForecastKind discriminates the kind of forecast.
type ForecastKind string

const (
	ForecastKindLoad    ForecastKind = "load"
	ForecastKindPv      ForecastKind = "pv"
	ForecastKindBattery ForecastKind = "battery"
	ForecastKindGas     ForecastKind = "gas"
)

// Forecasts is the global event handler for forecast updates.
// Listeners receive both newly generated forecasts and the per-bucket "active" replays.
var Forecasts = genericEventHandler[ForecastChangeListener, *ForecastValues]{
	listeners: make(map[ForecastChangeListener]func(values *ForecastValues) bool),
}

type ForecastChangeListener interface {
	HandleEvent(*ForecastValues)
}

// ForecastValues describes a single forecast bucket.
type ForecastValues struct {
	eventTime   time.Time
	generatedAt time.Time
	modelName   string
	sourceName  string
	kind        ForecastKind
	bucketStart time.Time
	bucketSize  time.Duration
	watts       float32
	confidence  float32
	active      bool
}

func NewForecastValues() *ForecastValues {
	return &ForecastValues{eventTime: time.Now()}
}

func (f *ForecastValues) EventTime() time.Time   { return f.eventTime }
func (f *ForecastValues) GeneratedAt() time.Time { return f.generatedAt }
func (f *ForecastValues) ModelName() string      { return f.modelName }
func (f *ForecastValues) Kind() ForecastKind     { return f.kind }
func (f *ForecastValues) SourceName() string     { return f.sourceName }
func (f *ForecastValues) BucketStart() time.Time { return f.bucketStart }
func (f *ForecastValues) BucketSize() time.Duration {
	return f.bucketSize
}
func (f *ForecastValues) Watts() float32      { return f.watts }
func (f *ForecastValues) Confidence() float32 { return f.confidence }

// Active reports whether this is a per-bucket "active" replay event (true)
// or a freshly generated forecast notification (false).
func (f *ForecastValues) Active() bool { return f.active }

func (f *ForecastValues) SetGeneratedAt(t time.Time) *ForecastValues {
	f.generatedAt = t
	return f
}
func (f *ForecastValues) SetModelName(name string) *ForecastValues {
	f.modelName = name
	return f
}
func (f *ForecastValues) SetKind(kind ForecastKind) *ForecastValues {
	f.kind = kind
	return f
}
func (f *ForecastValues) SetSourceName(name string) *ForecastValues {
	f.sourceName = name
	return f
}
func (f *ForecastValues) SetBucketStart(t time.Time) *ForecastValues {
	f.bucketStart = t
	return f
}
func (f *ForecastValues) SetBucketSize(size time.Duration) *ForecastValues {
	f.bucketSize = size
	return f
}
func (f *ForecastValues) SetWatts(w float32) *ForecastValues {
	f.watts = w
	return f
}
func (f *ForecastValues) SetConfidence(c float32) *ForecastValues {
	f.confidence = c
	return f
}
func (f *ForecastValues) SetActive(active bool) *ForecastValues {
	f.active = active
	return f
}

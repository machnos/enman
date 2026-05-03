package events

import "time"

// BatteryAction enumerates the possible actions the optimizer plans for a single bucket.
type BatteryAction string

const (
	BatteryActionIdle            BatteryAction = "idle"
	BatteryActionChargeFromGrid  BatteryAction = "charge_from_grid"
	BatteryActionChargeFromPv    BatteryAction = "charge_from_pv"
	BatteryActionDischargeToHome BatteryAction = "discharge_to_home"
	BatteryActionDischargeToGrid BatteryAction = "discharge_to_grid"
	BatteryActionExportOnly      BatteryAction = "export_only"
)

// BatterySchedules is the global event handler for battery-schedule updates.
// Listeners receive both freshly generated schedules and per-bucket "active" replays.
var BatterySchedules = genericEventHandler[BatteryScheduleChangeListener, *BatteryScheduleValues]{
	listeners: make(map[BatteryScheduleChangeListener]func(values *BatteryScheduleValues) bool),
}

type BatteryScheduleChangeListener interface {
	HandleEvent(*BatteryScheduleValues)
}

// BatteryScheduleValues describes a single planned action for a single bucket.
type BatteryScheduleValues struct {
	eventTime    time.Time
	generatedAt  time.Time
	batteryName  string
	bucketStart  time.Time
	bucketSize   time.Duration
	action       BatteryAction
	powerW       float32
	predictedSoC float32
	reason       string
	active       bool
}

func NewBatteryScheduleValues() *BatteryScheduleValues {
	return &BatteryScheduleValues{eventTime: time.Now()}
}

func (b *BatteryScheduleValues) EventTime() time.Time      { return b.eventTime }
func (b *BatteryScheduleValues) GeneratedAt() time.Time    { return b.generatedAt }
func (b *BatteryScheduleValues) BatteryName() string       { return b.batteryName }
func (b *BatteryScheduleValues) BucketStart() time.Time    { return b.bucketStart }
func (b *BatteryScheduleValues) BucketSize() time.Duration { return b.bucketSize }
func (b *BatteryScheduleValues) Action() BatteryAction     { return b.action }

// PowerW is signed: positive means charge into the battery, negative means discharge.
func (b *BatteryScheduleValues) PowerW() float32       { return b.powerW }
func (b *BatteryScheduleValues) PredictedSoC() float32 { return b.predictedSoC }
func (b *BatteryScheduleValues) Reason() string        { return b.reason }
func (b *BatteryScheduleValues) Active() bool          { return b.active }

func (b *BatteryScheduleValues) SetGeneratedAt(t time.Time) *BatteryScheduleValues {
	b.generatedAt = t
	return b
}
func (b *BatteryScheduleValues) SetBatteryName(name string) *BatteryScheduleValues {
	b.batteryName = name
	return b
}
func (b *BatteryScheduleValues) SetBucketStart(t time.Time) *BatteryScheduleValues {
	b.bucketStart = t
	return b
}
func (b *BatteryScheduleValues) SetBucketSize(size time.Duration) *BatteryScheduleValues {
	b.bucketSize = size
	return b
}
func (b *BatteryScheduleValues) SetAction(a BatteryAction) *BatteryScheduleValues {
	b.action = a
	return b
}
func (b *BatteryScheduleValues) SetPowerW(p float32) *BatteryScheduleValues {
	b.powerW = p
	return b
}
func (b *BatteryScheduleValues) SetPredictedSoC(soc float32) *BatteryScheduleValues {
	b.predictedSoC = soc
	return b
}
func (b *BatteryScheduleValues) SetReason(reason string) *BatteryScheduleValues {
	b.reason = reason
	return b
}
func (b *BatteryScheduleValues) SetActive(active bool) *BatteryScheduleValues {
	b.active = active
	return b
}

package battery

import (
	"encoding/json"
	"time"
)

// ChargingSource indicates the source of energy for charging
type ChargingSource uint8

const (
	// ChargingSourceNone indicates no charging is happening
	ChargingSourceNone ChargingSource = iota
	// ChargingSourceGrid indicates charging from the grid
	ChargingSourceGrid
	// ChargingSourcePV indicates charging from solar PV (excess production)
	ChargingSourcePV
)

func (c ChargingSource) String() string {
	switch c {
	case ChargingSourceGrid:
		return "grid"
	case ChargingSourcePV:
		return "pv"
	default:
		return "none"
	}
}

// ScheduleSlot represents a single time slot in the battery schedule.
// Each slot covers a specific time period and contains the target
// power for charging or discharging the battery.
type ScheduleSlot struct {
	// startTime is the start of this time slot (inclusive)
	startTime time.Time
	// endTime is the end of this time slot (exclusive)
	endTime time.Time
	// chargePower is the target power in watts
	// Positive = charging from grid, Negative = discharging to grid/household
	chargePower float32
	// predictedSoC is the expected state of charge at the end of this slot (0-100)
	predictedSoC float32
	// pricePerKwh is the electricity price during this slot (for reference/visualization)
	pricePerKwh float32
	// chargingSource indicates the source of energy for charging (grid or PV)
	chargingSource ChargingSource
}

// NewScheduleSlot creates a new schedule slot
func NewScheduleSlot(startTime, endTime time.Time) *ScheduleSlot {
	return &ScheduleSlot{
		startTime: startTime,
		endTime:   endTime,
	}
}

// StartTime returns the start time of this slot
func (s *ScheduleSlot) StartTime() time.Time {
	return s.startTime
}

// EndTime returns the end time of this slot
func (s *ScheduleSlot) EndTime() time.Time {
	return s.endTime
}

// Duration returns the duration of this slot
func (s *ScheduleSlot) Duration() time.Duration {
	return s.endTime.Sub(s.startTime)
}

// ChargePower returns the target power in watts
// Positive = charging from grid, Negative = discharging to grid/household
func (s *ScheduleSlot) ChargePower() float32 {
	return s.chargePower
}

// SetChargePower sets the target power in watts
// Positive = charging from grid, Negative = discharging to grid/household
func (s *ScheduleSlot) SetChargePower(power float32) *ScheduleSlot {
	s.chargePower = power
	return s
}

// PredictedSoC returns the expected state of charge at the end of this slot
func (s *ScheduleSlot) PredictedSoC() float32 {
	return s.predictedSoC
}

// SetPredictedSoC sets the expected state of charge at the end of this slot
func (s *ScheduleSlot) SetPredictedSoC(soc float32) *ScheduleSlot {
	s.predictedSoC = soc
	return s
}

// PricePerKwh returns the electricity price during this slot
func (s *ScheduleSlot) PricePerKwh() float32 {
	return s.pricePerKwh
}

// SetPricePerKwh sets the electricity price during this slot
func (s *ScheduleSlot) SetPricePerKwh(price float32) *ScheduleSlot {
	s.pricePerKwh = price
	return s
}

// ChargingSource returns the source of energy for charging (grid or PV)
func (s *ScheduleSlot) ChargingSource() ChargingSource {
	return s.chargingSource
}

// SetChargingSource sets the source of energy for charging
func (s *ScheduleSlot) SetChargingSource(source ChargingSource) *ScheduleSlot {
	s.chargingSource = source
	return s
}

// IsActive returns true if the current time falls within this slot
func (s *ScheduleSlot) IsActive() bool {
	now := time.Now()
	return !now.Before(s.startTime) && now.Before(s.endTime)
}

// IsCharging returns true if this slot is for charging the battery
func (s *ScheduleSlot) IsCharging() bool {
	return s.chargePower > 0
}

// IsDischarging returns true if this slot is for discharging the battery
func (s *ScheduleSlot) IsDischarging() bool {
	return s.chargePower < 0
}

// IsIdle returns true if this slot has no charging or discharging
func (s *ScheduleSlot) IsIdle() bool {
	return s.chargePower == 0
}

// Energy returns the expected energy transfer during this slot in kWh
// Positive = energy charged, Negative = energy discharged
func (s *ScheduleSlot) Energy() float32 {
	hours := float32(s.Duration().Hours())
	return (s.chargePower * hours) / 1000
}

// scheduleSlot is the internal struct for JSON serialization
type scheduleSlot struct {
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	ChargePower    float32   `json:"charge_power"`
	PredictedSoC   float32   `json:"predicted_soc"`
	PricePerKwh    float32   `json:"price_per_kwh"`
	ChargingSource string    `json:"charging_source"`
}

// MarshalJSON implements json.Marshaler
func (s *ScheduleSlot) MarshalJSON() ([]byte, error) {
	return json.Marshal(scheduleSlot{
		StartTime:      s.startTime,
		EndTime:        s.endTime,
		ChargePower:    s.chargePower,
		PredictedSoC:   s.predictedSoC,
		PricePerKwh:    s.pricePerKwh,
		ChargingSource: s.chargingSource.String(),
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (s *ScheduleSlot) UnmarshalJSON(data []byte) error {
	var raw scheduleSlot
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.startTime = raw.StartTime
	s.endTime = raw.EndTime
	s.chargePower = raw.ChargePower
	s.predictedSoC = raw.PredictedSoC
	s.pricePerKwh = raw.PricePerKwh
	switch raw.ChargingSource {
	case "grid":
		s.chargingSource = ChargingSourceGrid
	case "pv":
		s.chargingSource = ChargingSourcePV
	default:
		s.chargingSource = ChargingSourceNone
	}
	return nil
}

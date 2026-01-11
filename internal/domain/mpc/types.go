package mpc

import (
	"time"
)

// NumTimeSteps defines the planning horizon (24 hours with 1-hour resolution)
const NumTimeSteps = 24

// OptimizationInput contains all input data needed for MPC optimization
type OptimizationInput struct {
	// Current battery state
	CurrentSoC  float32   // Current State of Charge (%)
	CurrentTime time.Time // Current timestamp

	// Battery parameters
	BatteryRoundTripEff float32 // Round-trip efficiency (%) - e.g., 90 means 90%
	BatteryCapacity     float32 // Total capacity in kWh
	MaxChargePower      float32 // Maximum charging power in Watts
	MaxDischargePower   float32 // Maximum discharging power in Watts
	MinSoC              float32 // Minimum allowed SoC (%)
	MaxSoC              float32 // Maximum allowed SoC (%)

	// Price forecast for next 24 hours (hourly resolution)
	EnergyPrices []PricePoint // Energy prices per time step

	// Demand forecast (hourly consumption in Watts)
	ForecastedConsumption []float32

	// PV production forecast (hourly production in Watts)
	ForecastedPVProduction []float32
}

// PricePoint represents electricity price for a time window
type PricePoint struct {
	Time             time.Time
	EndTime          time.Time
	ConsumptionPrice float32 // Price for buying electricity (€/kWh)
	FeedbackPrice    float32 // Price for selling electricity (€/kWh)
}

// OptimizationResult contains the output of MPC optimization
type OptimizationResult struct {
	Success       bool
	Message       string
	OptimalAction float32   // Power to charge (positive) or discharge (negative) in Watts for current timestep
	TotalCost     float32   // Total predicted cost for the horizon
	SoCTrajectory []float32 // Predicted SoC trajectory over the horizon (%)

	// Detailed schedule for analysis/debugging
	ChargingSchedule    []float32 // Planned charging power per timestep (Watts)
	DischargingSchedule []float32 // Planned discharging power per timestep (Watts)
	GridImportSchedule  []float32 // Planned grid import per timestep (Watts)
	GridExportSchedule  []float32 // Planned grid export per timestep (Watts)
}

// HistoricalData contains aggregated historical energy data
type HistoricalData struct {
	// Hourly averages for 24 hours (0-23)
	HourlyAverages [24]float32

	// Day of week patterns (0=Sunday, 6=Saturday)
	DayOfWeekFactors [7]float32

	// Seasonal factor (optional)
	SeasonalFactor float32

	// Data quality metrics
	DataPointCount int
	LastUpdated    time.Time
}

// PVProductionData contains historical PV production data
type PVProductionData struct {
	// Hourly averages for 24 hours (0-23)
	HourlyAverages [24]float32

	// Month-specific scaling factors (0=Jan, 11=Dec)
	MonthlyFactors [12]float32

	// Data quality
	DataPointCount int
	LastUpdated    time.Time
}

// GridConsumptionData contains historical grid consumption data
type GridConsumptionData struct {
	// Hourly averages for 24 hours (0-23)
	HourlyAverages [24]float32

	// Day of week patterns
	DayOfWeekFactors [7]float32

	// Weekend vs weekday factor
	WeekendFactor float32

	// Data quality
	DataPointCount int
	LastUpdated    time.Time
}

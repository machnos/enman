package mpc

import (
	"math"
)

// MPCOptimizer implements Model Predictive Control for battery charging optimization
// using Mixed-Integer Linear Programming (MILP) to find the cost-optimal charging schedule
type MPCOptimizer struct {
	minSoC float32
	maxSoC float32
}

// NewMPCOptimizer creates a new MPC optimizer
func NewMPCOptimizer(minSoC, maxSoC float32) *MPCOptimizer {
	return &MPCOptimizer{
		minSoC: minSoC,
		maxSoC: maxSoC,
	}
}

// Optimize runs the MPC optimization to find the optimal battery charging schedule
// It uses a MILP formulation to minimize total electricity cost over the planning horizon
func (m *MPCOptimizer) Optimize(input *OptimizationInput) *OptimizationResult {
	// Validate input
	if err := m.validateInput(input); err != nil {
		return &OptimizationResult{
			Success: false,
			Message: err.Error(),
		}
	}

	// Build and solve the MILP problem
	problem := m.buildProblem(input)
	solution := m.solveMILP(problem)

	if !solution.feasible {
		return &OptimizationResult{
			Success: false,
			Message: "No feasible solution found",
		}
	}

	// Extract results
	result := m.extractResult(solution, input)
	return result
}

// MILPProblem represents the MILP optimization problem
type MILPProblem struct {
	numTimesteps int

	// Battery parameters
	batteryCapacityKWh  float32
	chargeEfficiency    float32 // sqrt of round-trip efficiency
	dischargeEfficiency float32 // sqrt of round-trip efficiency
	maxChargePowerKW    float32
	maxDischargePowerKW float32
	minSoC              float32
	maxSoC              float32
	initialSoC          float32

	// Forecast data
	prices       []PricePoint
	consumption  []float32 // Watts
	pvProduction []float32 // Watts

	// Time step duration in hours
	timestepHours float32
}

// MILPSolution represents the solution to the MILP problem
type MILPSolution struct {
	feasible bool

	// Decision variables
	chargePower    []float32 // Charging power per timestep (kW)
	dischargePower []float32 // Discharging power per timestep (kW)
	gridImport     []float32 // Grid import per timestep (kW)
	gridExport     []float32 // Grid export per timestep (kW)
	soc            []float32 // State of charge at end of each timestep (%)

	// Objective
	totalCost float32
}

func (m *MPCOptimizer) validateInput(input *OptimizationInput) error {
	if input.BatteryCapacity <= 0 {
		return &OptimizationError{Message: "Battery capacity must be positive"}
	}
	if len(input.EnergyPrices) == 0 {
		return &OptimizationError{Message: "Energy prices are required"}
	}
	if input.CurrentSoC < 0 || input.CurrentSoC > 100 {
		return &OptimizationError{Message: "Current SoC must be between 0 and 100"}
	}
	return nil
}

type OptimizationError struct {
	Message string
}

func (e *OptimizationError) Error() string {
	return e.Message
}

func (m *MPCOptimizer) buildProblem(input *OptimizationInput) *MILPProblem {
	numTimesteps := min(len(input.EnergyPrices), NumTimeSteps)

	// Calculate charge/discharge efficiency from round-trip efficiency
	// Round-trip efficiency = charge_eff * discharge_eff
	// Assuming symmetric: each = sqrt(round_trip)
	rtEff := input.BatteryRoundTripEff / 100.0
	if rtEff <= 0 || rtEff > 1 {
		rtEff = 0.9 // Default 90%
	}
	singleEff := float32(math.Sqrt(float64(rtEff)))

	problem := &MILPProblem{
		numTimesteps:        numTimesteps,
		batteryCapacityKWh:  input.BatteryCapacity,
		chargeEfficiency:    singleEff,
		dischargeEfficiency: singleEff,
		maxChargePowerKW:    input.MaxChargePower / 1000.0,    // W to kW
		maxDischargePowerKW: input.MaxDischargePower / 1000.0, // W to kW
		minSoC:              input.MinSoC,
		maxSoC:              input.MaxSoC,
		initialSoC:          input.CurrentSoC,
		prices:              input.EnergyPrices,
		consumption:         make([]float32, numTimesteps),
		pvProduction:        make([]float32, numTimesteps),
		timestepHours:       1.0, // 1-hour timesteps
	}

	// Copy consumption and PV forecasts (convert to kW)
	for i := 0; i < numTimesteps; i++ {
		if i < len(input.ForecastedConsumption) {
			problem.consumption[i] = input.ForecastedConsumption[i] / 1000.0
		}
		if i < len(input.ForecastedPVProduction) {
			problem.pvProduction[i] = input.ForecastedPVProduction[i] / 1000.0
		}
	}

	return problem
}

// solveMILP solves the MILP problem using the specialized MILP solver
// which uses dynamic programming for optimal battery scheduling
func (m *MPCOptimizer) solveMILP(problem *MILPProblem) *MILPSolution {
	solver := NewMILPSolver()
	return solver.Solve(problem)
}

func (m *MPCOptimizer) extractResult(solution *MILPSolution, input *OptimizationInput) *OptimizationResult {
	result := &OptimizationResult{
		Success:             solution.feasible,
		Message:             "Optimization successful",
		TotalCost:           solution.totalCost,
		SoCTrajectory:       solution.soc,
		ChargingSchedule:    make([]float32, len(solution.chargePower)),
		DischargingSchedule: make([]float32, len(solution.dischargePower)),
		GridImportSchedule:  make([]float32, len(solution.gridImport)),
		GridExportSchedule:  make([]float32, len(solution.gridExport)),
	}

	// Convert from kW back to W
	for i := range solution.chargePower {
		result.ChargingSchedule[i] = solution.chargePower[i] * 1000
		result.DischargingSchedule[i] = solution.dischargePower[i] * 1000
		result.GridImportSchedule[i] = solution.gridImport[i] * 1000
		result.GridExportSchedule[i] = solution.gridExport[i] * 1000
	}

	// The optimal action is the charging power for the first timestep
	// Positive = charge from grid, Negative = discharge to grid
	if len(solution.chargePower) > 0 {
		result.OptimalAction = solution.chargePower[0] * 1000 // Convert to Watts
	}

	return result
}

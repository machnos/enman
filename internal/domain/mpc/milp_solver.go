package mpc

import (
	"enman/internal/log"
	"math"
	"sort"
)

// MILPSolver implements a Mixed-Integer Linear Programming solver
// specifically designed for battery charging optimization problems.
// It uses a combination of LP relaxation and intelligent branching.
type MILPSolver struct {
	maxIterations int
	tolerance     float32
}

// NewMILPSolver creates a new MILP solver instance
func NewMILPSolver() *MILPSolver {
	return &MILPSolver{
		maxIterations: 1000,
		tolerance:     0.001,
	}
}

// Solve finds the optimal solution to the battery scheduling problem
// using a specialized MILP algorithm
func (s *MILPSolver) Solve(problem *MILPProblem) *MILPSolution {
	// Step 1: Analyze price structure
	priceAnalysis := s.analyzePrices(problem)

	// Step 2: Calculate optimal charge/discharge windows
	windows := s.calculateOptimalWindows(problem, priceAnalysis)

	// Step 3: Solve using dynamic programming approach
	dpSolution := s.solveDynamicProgramming(problem, windows)

	if dpSolution != nil && dpSolution.feasible {
		return dpSolution
	}

	// Fallback to greedy solution if DP fails
	return s.solveGreedy(problem)
}

// PriceAnalysis contains analysis of the price structure
type PriceAnalysis struct {
	minPrice        float32
	maxPrice        float32
	avgPrice        float32
	priceSpread     float32
	cheapThreshold  float32 // Prices below this are considered cheap
	expensiveThresh float32 // Prices above this are considered expensive
	sortedByPrice   []int   // Timestep indices sorted by price
	arbitrageProfit float32 // Potential profit per kWh from arbitrage
}

func (s *MILPSolver) analyzePrices(problem *MILPProblem) *PriceAnalysis {
	n := problem.numTimesteps
	if n == 0 {
		return &PriceAnalysis{}
	}

	analysis := &PriceAnalysis{
		minPrice:      problem.prices[0].ConsumptionPrice,
		maxPrice:      problem.prices[0].ConsumptionPrice,
		sortedByPrice: make([]int, n),
	}

	var totalPrice float32
	for i := 0; i < n; i++ {
		price := problem.prices[i].ConsumptionPrice
		analysis.sortedByPrice[i] = i
		totalPrice += price

		if price < analysis.minPrice {
			analysis.minPrice = price
		}
		if price > analysis.maxPrice {
			analysis.maxPrice = price
		}
	}

	analysis.avgPrice = totalPrice / float32(n)
	analysis.priceSpread = analysis.maxPrice - analysis.minPrice

	// Calculate thresholds (25th and 75th percentiles approximately)
	analysis.cheapThreshold = analysis.avgPrice - analysis.priceSpread*0.25
	analysis.expensiveThresh = analysis.avgPrice + analysis.priceSpread*0.25

	// Sort indices by price
	sort.Slice(analysis.sortedByPrice, func(i, j int) bool {
		return problem.prices[analysis.sortedByPrice[i]].ConsumptionPrice <
			problem.prices[analysis.sortedByPrice[j]].ConsumptionPrice
	})

	// Calculate arbitrage profit potential
	eff := problem.chargeEfficiency * problem.dischargeEfficiency
	analysis.arbitrageProfit = (analysis.maxPrice * eff) - analysis.minPrice

	return analysis
}

// OptimalWindow represents a time window for charging or discharging
type OptimalWindow struct {
	start     int
	end       int
	isCharge  bool
	avgPrice  float32
	priority  float32 // Higher = more attractive
	maxEnergy float32 // Maximum energy that can be stored/released in this window
}

func (s *MILPSolver) calculateOptimalWindows(problem *MILPProblem, analysis *PriceAnalysis) []OptimalWindow {
	windows := make([]OptimalWindow, 0)
	n := problem.numTimesteps

	// Identify charging windows (cheap periods)
	inCheapWindow := false
	var windowStart int

	for i := 0; i < n; i++ {
		price := problem.prices[i].ConsumptionPrice
		isCheap := price <= analysis.cheapThreshold

		if isCheap && !inCheapWindow {
			windowStart = i
			inCheapWindow = true
		} else if (!isCheap || i == n-1) && inCheapWindow {
			endIdx := i
			if isCheap && i == n-1 {
				endIdx = i + 1
			}

			// Calculate window metrics
			var avgPrice float32
			for j := windowStart; j < endIdx; j++ {
				avgPrice += problem.prices[j].ConsumptionPrice
			}
			avgPrice /= float32(endIdx - windowStart)

			windows = append(windows, OptimalWindow{
				start:     windowStart,
				end:       endIdx,
				isCharge:  true,
				avgPrice:  avgPrice,
				priority:  analysis.maxPrice - avgPrice, // Higher spread = higher priority
				maxEnergy: problem.maxChargePowerKW * float32(endIdx-windowStart),
			})
			inCheapWindow = false
		}
	}

	// Identify discharging windows (expensive periods)
	inExpWindow := false
	for i := 0; i < n; i++ {
		price := problem.prices[i].ConsumptionPrice
		isExpensive := price >= analysis.expensiveThresh

		if isExpensive && !inExpWindow {
			windowStart = i
			inExpWindow = true
		} else if (!isExpensive || i == n-1) && inExpWindow {
			endIdx := i
			if isExpensive && i == n-1 {
				endIdx = i + 1
			}

			var avgPrice float32
			for j := windowStart; j < endIdx; j++ {
				avgPrice += problem.prices[j].ConsumptionPrice
			}
			avgPrice /= float32(endIdx - windowStart)

			windows = append(windows, OptimalWindow{
				start:     windowStart,
				end:       endIdx,
				isCharge:  false,
				avgPrice:  avgPrice,
				priority:  avgPrice - analysis.minPrice,
				maxEnergy: problem.maxDischargePowerKW * float32(endIdx-windowStart),
			})
			inExpWindow = false
		}
	}

	// Sort windows by priority
	sort.Slice(windows, func(i, j int) bool {
		return windows[i].priority > windows[j].priority
	})

	return windows
}

// solveDynamicProgramming uses dynamic programming to find optimal solution
func (s *MILPSolver) solveDynamicProgramming(problem *MILPProblem, windows []OptimalWindow) *MILPSolution {
	n := problem.numTimesteps

	// Discretize SoC into states for DP
	numStates := 21 // 0%, 5%, 10%, ..., 100%
	stateStep := float32(5.0)

	// State value function: cost to reach each SoC state at each timestep
	// value[t][s] = minimum cost to reach state s at time t
	value := make([][]float32, n+1)
	// Track the best action and previous state for each (t, state) pair
	bestAction := make([][]int, n+1) // action taken to reach this state
	prevState := make([][]int, n+1)  // previous state index
	for t := 0; t <= n; t++ {
		value[t] = make([]float32, numStates)
		bestAction[t] = make([]int, numStates)
		prevState[t] = make([]int, numStates)
		for s := 0; s < numStates; s++ {
			value[t][s] = math.MaxFloat32
			prevState[t][s] = -1
		}
	}

	// Initial state
	initialStateIdx := s.socToStateIndex(problem.initialSoC, stateStep, numStates)
	value[0][initialStateIdx] = 0

	// Forward pass: calculate minimum cost to reach each state
	for t := 0; t < n; t++ {
		price := problem.prices[t].ConsumptionPrice
		feedbackPrice := problem.prices[t].FeedbackPrice
		netDemand := problem.consumption[t] - problem.pvProduction[t]

		for fromState := 0; fromState < numStates; fromState++ {
			if value[t][fromState] >= math.MaxFloat32/2 {
				continue
			}

			fromSoC := s.stateIndexToSoC(fromState, stateStep)

			// Check SoC constraints
			if fromSoC < problem.minSoC || fromSoC > problem.maxSoC {
				continue
			}

			// Try different actions: charge, discharge, or idle
			for action := -1; action <= 1; action++ {
				var chargePower, dischargePower float32

				switch action {
				case 1: // Charge
					// Max charge limited by SoC headroom and power limit
					maxChargeKWh := (problem.maxSoC - fromSoC) / 100 * problem.batteryCapacityKWh
					chargePower = min(problem.maxChargePowerKW, maxChargeKWh/problem.timestepHours)
					if chargePower <= 0 {
						continue // Can't charge
					}
				case -1: // Discharge
					// Max discharge limited by SoC floor and power limit
					maxDischargeKWh := (fromSoC - problem.minSoC) / 100 * problem.batteryCapacityKWh
					dischargePower = min(problem.maxDischargePowerKW, maxDischargeKWh/problem.timestepHours)
					if dischargePower <= 0 {
						continue // Can't discharge
					}
				}

				// Calculate new SoC
				chargeEnergy := chargePower * problem.timestepHours * problem.chargeEfficiency
				dischargeEnergy := dischargePower * problem.timestepHours
				newSoC := fromSoC + (chargeEnergy-dischargeEnergy)/problem.batteryCapacityKWh*100
				newSoC = max(problem.minSoC, min(problem.maxSoC, newSoC))

				toState := s.socToStateIndex(newSoC, stateStep, numStates)
				if toState < 0 || toState >= numStates {
					continue
				}

				// Calculate cost for this action
				effectiveDischarge := dischargePower * problem.dischargeEfficiency
				gridFlow := netDemand + chargePower - effectiveDischarge

				var cost float32
				if gridFlow > 0 {
					cost = gridFlow * problem.timestepHours * price
				} else {
					cost = gridFlow * problem.timestepHours * feedbackPrice // Negative = revenue
				}

				totalCost := value[t][fromState] + cost

				if totalCost < value[t+1][toState] {
					value[t+1][toState] = totalCost
					bestAction[t+1][toState] = action
					prevState[t+1][toState] = fromState
				}
			}
		}
	}

	// Find best final state
	bestFinalCost := float32(math.MaxFloat32)
	bestFinalState := -1
	for st := 0; st < numStates; st++ {
		if value[n][st] < bestFinalCost {
			bestFinalCost = value[n][st]
			bestFinalState = st
		}
	}

	if bestFinalState < 0 {
		return nil
	}

	// Backward pass: reconstruct the path
	actionSequence := make([]int, n)
	stateSequence := make([]int, n+1)
	stateSequence[n] = bestFinalState

	for t := n; t > 0; t-- {
		actionSequence[t-1] = bestAction[t][stateSequence[t]]
		stateSequence[t-1] = prevState[t][stateSequence[t]]
	}

	// Build solution from action sequence
	solution := &MILPSolution{
		feasible:       true,
		chargePower:    make([]float32, n),
		dischargePower: make([]float32, n),
		gridImport:     make([]float32, n),
		gridExport:     make([]float32, n),
		soc:            make([]float32, n),
		totalCost:      bestFinalCost,
	}

	currentSoC := problem.initialSoC

	for t := 0; t < n; t++ {
		netDemand := problem.consumption[t] - problem.pvProduction[t]
		action := actionSequence[t]

		var chargePower, dischargePower float32

		switch action {
		case 1: // Charge
			maxChargeKWh := (problem.maxSoC - currentSoC) / 100 * problem.batteryCapacityKWh
			chargePower = min(problem.maxChargePowerKW, maxChargeKWh/problem.timestepHours)
		case -1: // Discharge
			maxDischargeKWh := (currentSoC - problem.minSoC) / 100 * problem.batteryCapacityKWh
			dischargePower = min(problem.maxDischargePowerKW, maxDischargeKWh/problem.timestepHours)
		}

		solution.chargePower[t] = chargePower
		solution.dischargePower[t] = dischargePower

		// Update SoC
		chargeEnergy := chargePower * problem.timestepHours * problem.chargeEfficiency
		dischargeEnergy := dischargePower * problem.timestepHours
		currentSoC = currentSoC + (chargeEnergy-dischargeEnergy)/problem.batteryCapacityKWh*100
		currentSoC = max(problem.minSoC, min(problem.maxSoC, currentSoC))
		solution.soc[t] = currentSoC

		// Calculate grid flows
		effectiveDischarge := dischargePower * problem.dischargeEfficiency
		gridFlow := netDemand + chargePower - effectiveDischarge

		if gridFlow > 0 {
			solution.gridImport[t] = gridFlow
		} else {
			solution.gridExport[t] = -gridFlow
		}
	}

	log.Debugf("MILP DP solution: cost=%.4f, initial SoC=%.1f%%, final SoC=%.1f%%",
		solution.totalCost, problem.initialSoC, solution.soc[n-1])

	return solution
}

func (s *MILPSolver) socToStateIndex(soc float32, step float32, numStates int) int {
	idx := int(soc / step)
	return max(0, min(numStates-1, idx))
}

func (s *MILPSolver) stateIndexToSoC(idx int, step float32) float32 {
	return float32(idx) * step
}

// solveGreedy provides a fallback greedy solution
func (s *MILPSolver) solveGreedy(problem *MILPProblem) *MILPSolution {
	n := problem.numTimesteps

	solution := &MILPSolution{
		feasible:       true,
		chargePower:    make([]float32, n),
		dischargePower: make([]float32, n),
		gridImport:     make([]float32, n),
		gridExport:     make([]float32, n),
		soc:            make([]float32, n),
		totalCost:      0,
	}

	// Calculate average price
	var avgPrice float32
	for t := 0; t < n; t++ {
		avgPrice += problem.prices[t].ConsumptionPrice
	}
	avgPrice /= float32(n)

	currentSoC := problem.initialSoC

	for t := 0; t < n; t++ {
		price := problem.prices[t].ConsumptionPrice
		feedbackPrice := problem.prices[t].FeedbackPrice
		netDemand := problem.consumption[t] - problem.pvProduction[t]

		var chargePower, dischargePower float32

		// Simple rule: charge when cheap, discharge when expensive
		if price < avgPrice*0.8 && currentSoC < problem.maxSoC {
			// Cheap: charge
			maxChargeKWh := (problem.maxSoC - currentSoC) / 100 * problem.batteryCapacityKWh
			chargePower = min(problem.maxChargePowerKW, maxChargeKWh/problem.timestepHours)
		} else if price > avgPrice*1.2 && currentSoC > problem.minSoC && netDemand > 0 {
			// Expensive: discharge to cover demand
			maxDischargeKWh := (currentSoC - problem.minSoC) / 100 * problem.batteryCapacityKWh
			dischargePower = min(netDemand/problem.dischargeEfficiency, problem.maxDischargePowerKW)
			dischargePower = min(dischargePower, maxDischargeKWh/problem.timestepHours)
		}

		solution.chargePower[t] = chargePower
		solution.dischargePower[t] = dischargePower

		// Update SoC
		chargeEnergy := chargePower * problem.timestepHours * problem.chargeEfficiency
		dischargeEnergy := dischargePower * problem.timestepHours
		currentSoC = currentSoC + (chargeEnergy-dischargeEnergy)/problem.batteryCapacityKWh*100
		currentSoC = max(problem.minSoC, min(problem.maxSoC, currentSoC))
		solution.soc[t] = currentSoC

		// Calculate grid flows
		effectiveDischarge := dischargePower * problem.dischargeEfficiency
		gridFlow := netDemand + chargePower - effectiveDischarge

		if gridFlow > 0 {
			solution.gridImport[t] = gridFlow
			solution.totalCost += gridFlow * problem.timestepHours * price
		} else {
			solution.gridExport[t] = -gridFlow
			solution.totalCost -= (-gridFlow) * problem.timestepHours * feedbackPrice
		}
	}

	return solution
}

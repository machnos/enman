package optimizer

import (
	"context"
	"enman/internal/domain"
	"enman/internal/domain/battery"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"sort"
	"time"
)

// AcLoadFactor represents an AC load with its grid usage factor for consumption calculations
type AcLoadFactor struct {
	Name   string
	Role   constants.EnergySourceRole
	Factor float32 // 0.0 to 1.0: portion of consumption managed by optimizer (1 - percentageFromGrid/100)
}

// Config holds configuration for the battery schedule optimizer
type Config struct {
	// OptimizationHorizon is how far ahead to optimize (e.g., 24 hours)
	OptimizationHorizon time.Duration
	// SlotDuration is the duration of each schedule slot (e.g., 15 minutes)
	SlotDuration time.Duration
	// UpdateInterval is how often to recalculate the schedule (e.g., 5 minutes)
	UpdateInterval time.Duration
	// BatteryCapacityKwh is the total battery capacity in kWh (aggregated from all batteries)
	BatteryCapacityKwh float32
	// MaxChargePowerW is the maximum charging power in watts (aggregated from all batteries)
	MaxChargePowerW float32
	// MaxDischargePowerW is the maximum discharging power in watts (aggregated from all batteries)
	MaxDischargePowerW float32
	// MinSoC is the minimum state of charge to maintain (0-100)
	MinSoC float32
	// MaxSoC is the maximum state of charge (0-100)
	MaxSoC float32
	// RoundTripEfficiency is the battery round-trip efficiency (0-1, e.g., 0.9 for 90%)
	RoundTripEfficiency float32
	// EnergyProviderName is the name of the energy price provider
	EnergyProviderName string
	// BatteryNames is a list of battery source names in the repository
	BatteryNames []string
	// PvNames is a list of PV source names to consider for production prediction
	PvNames []string
	// AcLoadFactors contains AC loads with their optimization factor
	// Factor = 1 - (percentageFromGrid / 100): how much of the load is managed by the optimizer
	// e.g., percentageFromGrid=100 → factor=0 (fully excluded from optimization)
	// e.g., percentageFromGrid=50 → factor=0.5 (50% of consumption considered)
	// e.g., percentageFromGrid=0 → factor=1 (fully included in optimization)
	AcLoadFactors []AcLoadFactor
	// HistoricalDaysForPrediction is how many days of historical data to use for predictions
	HistoricalDaysForPrediction int
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		OptimizationHorizon:         24 * time.Hour,
		SlotDuration:                15 * time.Minute,
		UpdateInterval:              5 * time.Minute,
		BatteryCapacityKwh:          10.0,
		MaxChargePowerW:             5000,
		MaxDischargePowerW:          5000,
		MinSoC:                      10,
		MaxSoC:                      100,
		RoundTripEfficiency:         0.9,
		HistoricalDaysForPrediction: 7,
	}
}

// slotPrediction holds predicted values for a time slot
type slotPrediction struct {
	startTime             time.Time
	endTime               time.Time
	price                 float32 // Energy price per kWh
	predictedPvPowerW     float32 // Predicted PV production in watts
	predictedConsumptionW float32 // Predicted household consumption in watts
	netGridDemandW        float32 // Consumption minus PV (positive = need from grid)
}

// BatteryScheduleOptimizer generates optimal battery charge/discharge schedules
// using price-based optimization with constraints
type BatteryScheduleOptimizer struct {
	config     *Config
	repository repository.Repository
	ticker     *time.Ticker
	cancel     context.CancelFunc
}

// NewBatteryScheduleOptimizer creates a new battery schedule optimizer from a System
func NewBatteryScheduleOptimizer(
	system *domain.System,
	repo repository.Repository,
	roundTripEfficiency float32,
	minSoC float32,
) *BatteryScheduleOptimizer {
	config := DefaultConfig()

	// Extract grid name - used for both household consumption source and energy provider
	if system.Grid() != nil {
		config.EnergyProviderName = system.Grid().Name()
	}

	// Set SoC limits (max is always 100)
	if minSoC > 0 {
		config.MinSoC = minSoC
	}
	config.MaxSoC = 100

	if roundTripEfficiency > 0 && roundTripEfficiency <= 1 {
		config.RoundTripEfficiency = roundTripEfficiency
	}

	// Extract battery information from all batteries
	batteries := system.Batteries()
	config.BatteryNames = make([]string, 0, len(batteries))
	var totalCapacity float32
	var totalMaxCharge float32
	var totalMaxDischarge float32
	for _, b := range batteries {
		config.BatteryNames = append(config.BatteryNames, b.Name())
		// Capacity in kWh = Ah capacity * voltage / 1000
		totalCapacity += b.AvailableCapacity() * b.Voltage() / 1000
		totalMaxCharge += b.MaxChargePower()
		totalMaxDischarge += b.MaxDischargePower()
	}
	config.BatteryCapacityKwh = totalCapacity
	config.MaxChargePowerW = totalMaxCharge
	config.MaxDischargePowerW = totalMaxDischarge

	// Extract PV names from all PVs
	pvs := system.Pvs()
	config.PvNames = make([]string, 0, len(pvs))
	for _, pv := range pvs {
		config.PvNames = append(config.PvNames, pv.Name())
	}

	// Extract AC load factors from all AC loads
	// Factor = 1 - (percentageFromGrid / 100): portion managed by optimizer
	acLoads := system.AcLoads()
	config.AcLoadFactors = make([]AcLoadFactor, 0, len(acLoads))
	for _, acLoad := range acLoads {
		factor := 1.0 - float32(acLoad.PercentageFromGrid())/100.0
		config.AcLoadFactors = append(config.AcLoadFactors, AcLoadFactor{
			Name:   acLoad.Name(),
			Role:   acLoad.Role(),
			Factor: factor,
		})
	}

	return &BatteryScheduleOptimizer{
		config:     config,
		repository: repo,
	}
}

// NewBatteryScheduleOptimizerWithConfig creates a new optimizer with a custom config
// Use this for testing or when you need full control over configuration
func NewBatteryScheduleOptimizerWithConfig(config *Config, repo repository.Repository) *BatteryScheduleOptimizer {
	if config == nil {
		config = DefaultConfig()
	}
	return &BatteryScheduleOptimizer{
		config:     config,
		repository: repo,
	}
}

// Start begins the optimization loop
func (o *BatteryScheduleOptimizer) Start(ctx context.Context) {
	ctx, o.cancel = context.WithCancel(ctx)
	o.ticker = time.NewTicker(o.config.UpdateInterval)

	go func() {
		// Run initial optimization in the goroutine to avoid blocking startup
		o.runOptimization()

		for {
			select {
			case <-ctx.Done():
				o.ticker.Stop()
				return
			case <-o.ticker.C:
				o.runOptimization()
			}
		}
	}()
}

// Stop stops the optimization loop
func (o *BatteryScheduleOptimizer) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
}

// Config returns the current configuration
func (o *BatteryScheduleOptimizer) Config() *Config {
	return o.config
}

// getCurrentSoC fetches the current battery state of charge from the repository
// When multiple batteries exist, returns weighted average SoC based on capacity
func (o *BatteryScheduleOptimizer) getCurrentSoC() float32 {
	if o.repository == nil || len(o.config.BatteryNames) == 0 {
		log.Debug("Battery schedule optimizer: getCurrentSoC - repository is nil or no battery names configured")
		return 50 // Default fallback
	}

	var totalWeightedSoC float32
	var totalCapacity float32
	batteryCount := 0

	for _, batteryName := range o.config.BatteryNames {
		log.Debugf("Battery schedule optimizer: querying battery state for '%s'", batteryName)
		batteryState, err := o.repository.BatteryStateAtTime(
			time.Now(),
			batteryName,
			constants.EnergySourceRoleBattery,
			repository.LessOrEqual,
		)
		if err != nil {
			log.Debugf("Battery schedule optimizer: error querying battery '%s': %v", batteryName, err)
			continue
		}
		if batteryState == nil {
			log.Debugf("Battery schedule optimizer: no state found for battery '%s'", batteryName)
			continue
		}
		if batteryState.State == nil {
			log.Debugf("Battery schedule optimizer: state object is nil for battery '%s'", batteryName)
			continue
		}

		log.Debugf("Battery schedule optimizer: found SoC %.1f%% for battery '%s'", batteryState.State.SoC(), batteryName)
		// Use capacity from state if available, otherwise assume equal weighting
		// For weighted average, we'd need capacity info - for now use equal weights
		totalWeightedSoC += batteryState.State.SoC()
		totalCapacity += 1 // Equal weight per battery
		batteryCount++
	}

	if batteryCount == 0 {
		log.Debug("Could not fetch current battery state from any battery, using default SoC")
		return 50
	}

	return totalWeightedSoC / totalCapacity
}

// getPredictedPvProduction predicts PV production for a future time slot
// based on historical production at the same time of day
func (o *BatteryScheduleOptimizer) getPredictedPvProduction(slotStart, slotEnd time.Time) float32 {
	if o.repository == nil || len(o.config.PvNames) == 0 {
		return 0
	}

	var totalPrediction float32
	daysToCheck := o.config.HistoricalDaysForPrediction
	if daysToCheck <= 0 {
		daysToCheck = 7
	}

	// Look at the same time slot in previous days
	for _, pvName := range o.config.PvNames {
		var sumPower float32
		var count int

		for daysAgo := 1; daysAgo <= daysToCheck; daysAgo++ {
			historicalStart := slotStart.AddDate(0, 0, -daysAgo)
			historicalEnd := slotEnd.AddDate(0, 0, -daysAgo)

			states, err := o.repository.ElectricityStates(
				historicalStart,
				historicalEnd,
				pvName,
				&repository.AggregateConfiguration{
					WindowUnit:   repository.WindowUnitMinute,
					WindowAmount: uint64(o.config.SlotDuration.Minutes()),
					Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
				},
			)
			if err != nil || len(states) == 0 {
				continue
			}

			for _, stateRecord := range states {
				if avg, ok := stateRecord.States[repository.AggregateFunctionMean]; ok && avg != nil {
					// PV production is typically negative power (feeding back)
					// We want the absolute power value
					power := avg.TotalPower()
					if power < 0 {
						power = -power
					}
					sumPower += power
					count++
				}
			}
		}

		if count > 0 {
			totalPrediction += sumPower / float32(count)
		}
	}

	return totalPrediction
}

// getPredictedConsumption predicts household consumption for a future time slot
// based on historical consumption at the same time of day.
// Total household consumption = grid consumption + battery discharge power
// This accounts for the fact that when batteries are discharging, grid consumption
// is low but the household is still consuming power from the battery.
// Excludes AC loads that have percentageFromGrid=100 (user-managed loads like EV chargers)
func (o *BatteryScheduleOptimizer) getPredictedConsumption(slotStart, slotEnd time.Time) float32 {
	if o.repository == nil || o.config.EnergyProviderName == "" {
		return 0
	}

	daysToCheck := o.config.HistoricalDaysForPrediction
	if daysToCheck <= 0 {
		daysToCheck = 7
	}

	var sumPower float32
	var count int

	for daysAgo := 1; daysAgo <= daysToCheck; daysAgo++ {
		historicalStart := slotStart.AddDate(0, 0, -daysAgo)
		historicalEnd := slotEnd.AddDate(0, 0, -daysAgo)

		// Get grid consumption
		gridPower := float32(0)
		states, err := o.repository.ElectricityStates(
			historicalStart,
			historicalEnd,
			o.config.EnergyProviderName,
			&repository.AggregateConfiguration{
				WindowUnit:   repository.WindowUnitMinute,
				WindowAmount: uint64(o.config.SlotDuration.Minutes()),
				Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
			},
		)
		if err == nil && len(states) > 0 {
			for _, stateRecord := range states {
				if avg, ok := stateRecord.States[repository.AggregateFunctionMean]; ok && avg != nil {
					// Grid consumption is positive when consuming from grid
					power := avg.TotalPower()
					if power > 0 {
						gridPower += power
					}
				}
			}
		}

		// Get battery discharge power (negative power = discharging to household)
		batteryDischargePower := o.getBatteryDischargePower(historicalStart, historicalEnd)

		// Total household consumption = grid consumption + battery discharge
		totalPower := gridPower + batteryDischargePower

		if totalPower > 0 {
			// Subtract the portion of AC load consumption not managed by optimizer
			excludedPower := o.getAcLoadExcludedPower(historicalStart, historicalEnd)
			totalPower -= excludedPower
			if totalPower < 0 {
				totalPower = 0
			}
			sumPower += totalPower
			count++
		}
	}

	if count > 0 {
		return sumPower / float32(count)
	}
	return 0
}

// getBatteryDischargePower gets the average battery discharge power for a time slot.
// Returns positive value representing power flowing from batteries to household.
// Battery power is negative when discharging, so we negate it.
func (o *BatteryScheduleOptimizer) getBatteryDischargePower(slotStart, slotEnd time.Time) float32 {
	if len(o.config.BatteryNames) == 0 {
		return 0
	}

	var totalDischargePower float32

	for _, batteryName := range o.config.BatteryNames {
		states, err := o.repository.BatteryStates(
			slotStart,
			slotEnd,
			batteryName,
			&repository.AggregateConfiguration{
				WindowUnit:   repository.WindowUnitMinute,
				WindowAmount: uint64(o.config.SlotDuration.Minutes()),
				Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
			},
		)
		if err != nil || len(states) == 0 {
			continue
		}

		for _, stateRecord := range states {
			if avg, ok := stateRecord.States[repository.AggregateFunctionMean]; ok && avg != nil {
				// Battery power is negative when discharging (power flowing out)
				// We want positive discharge power for consumption calculation
				power := avg.Power()
				if power < 0 {
					totalDischargePower += -power // Negate to get positive discharge value
				}
			}
		}
	}

	return totalDischargePower
}

// getAcLoadExcludedPower calculates the power from AC loads that should be excluded from predictions.
// For each AC load, we exclude the portion based on percentageFromGrid:
// - percentageFromGrid=100 → exclude 100% of consumption (factor=0, user manages manually)
// - percentageFromGrid=50 → exclude 50% of consumption (factor=0.5)
// - percentageFromGrid=0 → exclude 0% (factor=1, fully managed by optimizer)
func (o *BatteryScheduleOptimizer) getAcLoadExcludedPower(slotStart, slotEnd time.Time) float32 {
	if len(o.config.AcLoadFactors) == 0 {
		return 0
	}

	var totalExcludedPower float32

	for _, acLoad := range o.config.AcLoadFactors {
		// Skip loads fully managed by optimizer (factor=1, nothing to exclude)
		if acLoad.Factor >= 1.0 {
			continue
		}

		states, err := o.repository.ElectricityStates(
			slotStart,
			slotEnd,
			acLoad.Name,
			&repository.AggregateConfiguration{
				WindowUnit:   repository.WindowUnitMinute,
				WindowAmount: uint64(o.config.SlotDuration.Minutes()),
				Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
			},
		)
		if err != nil || len(states) == 0 {
			continue
		}

		for _, stateRecord := range states {
			if avg, ok := stateRecord.States[repository.AggregateFunctionMean]; ok && avg != nil {
				power := avg.TotalPower()
				if power > 0 {
					// Exclude the portion not managed by optimizer: power * (1 - factor)
					excludedPortion := power * (1 - acLoad.Factor)
					totalExcludedPower += excludedPortion
				}
			}
		}
	}

	return totalExcludedPower
}

// buildSlotPredictions creates predictions for each time slot in the optimization horizon
func (o *BatteryScheduleOptimizer) buildSlotPredictions(now time.Time, energyPrices []*prices.EnergyPrice) []*slotPrediction {
	predictions := make([]*slotPrediction, 0)

	for _, ep := range energyPrices {
		// Skip slots that have already ended
		if ep.EndTime.Before(now) || ep.EndTime.Equal(now) {
			continue
		}

		startTime := ep.Time
		if startTime.Before(now) {
			startTime = now
		}

		pvPower := o.getPredictedPvProduction(startTime, ep.EndTime)
		consumption := o.getPredictedConsumption(startTime, ep.EndTime)

		// Net grid demand: what the household needs from grid after PV
		// Positive = need power from grid, Negative = excess PV to export/store
		netDemand := consumption - pvPower

		predictions = append(predictions, &slotPrediction{
			startTime:             startTime,
			endTime:               ep.EndTime,
			price:                 ep.ConsumptionPrice,
			predictedPvPowerW:     pvPower,
			predictedConsumptionW: consumption,
			netGridDemandW:        netDemand,
		})
	}

	return predictions
}

// runOptimization performs the optimization and fires events for new schedule slots
func (o *BatteryScheduleOptimizer) runOptimization() {
	now := time.Now()
	horizonEnd := now.Add(o.config.OptimizationHorizon)

	if o.repository == nil {
		log.Warning("Battery schedule optimizer: repository is nil, skipping optimization")
		return
	}

	log.Debugf("Battery schedule optimizer: fetching prices for provider '%s' from %v to %v",
		o.config.EnergyProviderName, now, horizonEnd)

	// Fetch energy prices for the optimization horizon
	energyPrices, err := o.repository.EnergyPrices(
		now,
		horizonEnd,
		o.config.EnergyProviderName,
		prices.EnergyTypeElectricity,
	)
	if err != nil {
		log.Errorf("Failed to fetch energy prices for optimization: %v", err)
		return
	}

	if len(energyPrices) == 0 {
		log.Debugf("Battery schedule optimizer: no energy prices available for provider '%s' in horizon %v to %v",
			o.config.EnergyProviderName, now, horizonEnd)
		return
	}

	log.Debugf("Battery schedule optimizer: found %d energy prices", len(energyPrices))

	// Get current battery state
	currentSoC := o.getCurrentSoC()
	log.Debugf("Battery schedule optimizer: current SoC is %.1f%%", currentSoC)

	// Build predictions for each slot
	predictions := o.buildSlotPredictions(now, energyPrices)
	log.Debugf("Battery schedule optimizer: built %d slot predictions", len(predictions))

	// Generate schedule slots based on optimization
	slots := o.Optimize(currentSoC, predictions)

	log.Infof("Battery schedule optimizer: generated %d schedule slots", len(slots))

	// Fire events for each slot
	for _, slot := range slots {
		event := events.NewBatteryScheduleSlotEvent(slot)
		events.BatteryScheduleSlots.Trigger(event)
	}

	if log.DebugEnabled() {
		log.Debugf("Generated %d battery schedule slots for horizon %v to %v", len(slots), now, horizonEnd)
	}
}

// slotAction represents what action to take for a slot
type slotAction int

const (
	actionIdle slotAction = iota
	actionChargePV
	actionChargeGrid
	actionDischarge
)

// ensureBatterySurvival simulates the schedule forward and prevents the battery from
// hitting MinSoC at expensive times. It uses a two-phase approach:
// Phase 1: Try to reduce/remove discharge actions that would cause expensive emergency charging
// Phase 2: If still needed, add survival charging at the cheapest available slots
//
// The key insight is: if discharging during a €0.35 slot would require emergency charging
// at €0.35, it's not economical - better to stay idle during that discharge slot.
func (o *BatteryScheduleOptimizer) ensureBatterySurvival(
	predictions []*slotPrediction,
	slotActions []slotAction,
	currentSoC float32,
	chargeThreshold float32,
) {
	maxIterations := len(predictions) * 2 // Prevent infinite loops
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Find the survival problem: simulate forward and find where we hit MinSoC
		problem := o.findSurvivalProblem(predictions, slotActions, currentSoC)
		if problem == nil {
			// No problem found, we're done
			return
		}

		// Find the next charging slot after the problem
		nextChargeIdx := -1
		nextChargePrice := float32(0)
		for i := problem.problemSlotIdx + 1; i < len(slotActions); i++ {
			if slotActions[i] == actionChargeGrid {
				nextChargeIdx = i
				nextChargePrice = predictions[i].price
				break
			}
		}

		// Check if there's a discharge slot (peak) between the problem and the next charging slot
		// If there's no peak to survive for, we don't need survival charging - just reduce discharge
		hasPeakToSurvive := o.hasPeakBetween(slotActions, problem.problemSlotIdx, nextChargeIdx)

		if !hasPeakToSurvive {
			// No upcoming peak to discharge into - just reduce discharge to avoid hitting MinSoC
			reduced := o.reduceAnyDischarge(predictions, slotActions, problem.problemSlotIdx)
			if !reduced {
				log.Debugf("Battery schedule optimizer: no peak to survive and couldn't reduce discharge at slot %d", problem.problemSlotIdx)
				return
			}
			continue // Retry simulation
		}

		// There IS a peak to survive - use the two-phase approach

		// Phase 1: Consider reducing discharge to avoid expensive emergency charging
		// If we'd need to charge at price P to survive, don't discharge at slots with price <= P
		if problem.wouldNeedEmergencyCharging {
			// Find the most expensive discharge slot before the problem that has price <= emergency charge price
			reducedDischarge := o.reduceUnprofitableDischarge(predictions, slotActions, problem, nextChargePrice)
			if reducedDischarge {
				continue // Retry simulation
			}
		}

		// Phase 2: Add survival charging at cheapest available slot
		added := o.addSurvivalCharging(predictions, slotActions, problem.problemSlotIdx, chargeThreshold)
		if !added {
			// Last resort: reduce discharge slots even if they're profitable
			reduced := o.reduceAnyDischarge(predictions, slotActions, problem.problemSlotIdx)
			if !reduced {
				log.Debugf("Battery schedule optimizer: couldn't resolve survival problem at slot %d", problem.problemSlotIdx)
				return
			}
		}
	}
}

// hasPeakBetween checks if there's a discharge slot (peak) between startIdx and endIdx (exclusive).
// If endIdx is -1, it checks from startIdx to the end of the schedule.
// Returns true if there's at least one discharge slot in that range.
func (o *BatteryScheduleOptimizer) hasPeakBetween(slotActions []slotAction, startIdx, endIdx int) bool {
	if endIdx == -1 {
		endIdx = len(slotActions)
	}
	for i := startIdx + 1; i < endIdx; i++ {
		if slotActions[i] == actionDischarge {
			return true
		}
	}
	return false
}

// survivalProblem describes where and why the battery would hit MinSoC
type survivalProblem struct {
	problemSlotIdx             int
	socAtProblem               float32
	wouldNeedEmergencyCharging bool
	emergencyChargePrice       float32
}

// findSurvivalProblem simulates the schedule forward to find where battery hits MinSoC
func (o *BatteryScheduleOptimizer) findSurvivalProblem(
	predictions []*slotPrediction,
	slotActions []slotAction,
	currentSoC float32,
) *survivalProblem {
	simulatedSoC := currentSoC
	minSoCBuffer := o.config.MinSoC + 5 // Add 5% buffer

	for i := 0; i < len(predictions); i++ {
		p := predictions[i]
		action := slotActions[i]

		// If this is a charging slot and we're at low SoC, charging will help
		if action == actionChargeGrid && simulatedSoC <= minSoCBuffer {
			// We found a problem - we need emergency charging at this slot
			return &survivalProblem{
				problemSlotIdx:             i,
				socAtProblem:               simulatedSoC,
				wouldNeedEmergencyCharging: true,
				emergencyChargePrice:       p.price,
			}
		}

		slotDurationHours := p.endTime.Sub(p.startTime).Hours()
		energyChangeKwh := o.calculateSlotEnergyChange(p, action, slotDurationHours)

		prevSoC := simulatedSoC
		simulatedSoC += energyChangeKwh / o.config.BatteryCapacityKwh * 100
		simulatedSoC = maxFloat32(o.config.MinSoC, minFloat32(o.config.MaxSoC, simulatedSoC))

		// Check if we hit MinSoC at an idle or discharge slot (not at a charge slot)
		if simulatedSoC <= minSoCBuffer && prevSoC > minSoCBuffer && action != actionChargeGrid {
			return &survivalProblem{
				problemSlotIdx:             i,
				socAtProblem:               simulatedSoC,
				wouldNeedEmergencyCharging: true,
				emergencyChargePrice:       p.price, // We'd need to charge at this price or higher
			}
		}
	}

	return nil
}

// calculateSlotEnergyChange calculates the energy change in kWh for a given slot action
func (o *BatteryScheduleOptimizer) calculateSlotEnergyChange(
	p *slotPrediction,
	action slotAction,
	slotDurationHours float64,
) float32 {
	var energyChangeKwh float32

	switch action {
	case actionChargePV:
		if p.netGridDemandW < 0 {
			excessPvW := -p.netGridDemandW
			maxChargePower := minFloat32(excessPvW, o.config.MaxChargePowerW)
			energyChangeKwh = maxChargePower / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		}
	case actionChargeGrid:
		maxChargeEnergyKwh := o.config.MaxChargePowerW / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		var consumptionDrain float32
		if p.netGridDemandW > 0 {
			consumptionDrain = p.netGridDemandW / 1000 * float32(slotDurationHours)
		}
		energyChangeKwh = maxChargeEnergyKwh - consumptionDrain
	case actionDischarge:
		if p.netGridDemandW > 0 {
			dischargePower := minFloat32(p.netGridDemandW, o.config.MaxDischargePowerW)
			energyChangeKwh = -dischargePower / 1000 * float32(slotDurationHours)
		} else {
			energyChangeKwh = -o.config.MaxDischargePowerW / 1000 * float32(slotDurationHours)
		}
	default: // actionIdle
		if p.netGridDemandW > 0 {
			energyChangeKwh = -p.netGridDemandW / 1000 * float32(slotDurationHours)
		}
	}

	return energyChangeKwh
}

// reduceUnprofitableDischarge reduces discharge at slots where the price is not significantly
// higher than the emergency charging price. It's not economical to discharge at €0.35
// if we then need to charge at €0.35 to survive.
func (o *BatteryScheduleOptimizer) reduceUnprofitableDischarge(
	predictions []*slotPrediction,
	slotActions []slotAction,
	problem *survivalProblem,
	nextChargePrice float32,
) bool {
	// Find the threshold: discharge is only profitable if price > charge price / efficiency
	// Adding 10% margin to account for battery wear
	chargePrice := problem.emergencyChargePrice
	if nextChargePrice > 0 && nextChargePrice < chargePrice {
		chargePrice = nextChargePrice
	}
	profitThreshold := chargePrice / o.config.RoundTripEfficiency * 1.1

	// Find discharge slots before the problem that are below the profit threshold
	// Convert the cheapest-to-discharge (lowest price discharge) to idle
	var cheapestDischargeIdx = -1
	var cheapestDischargePrice float32 = 999999

	for i := 0; i <= problem.problemSlotIdx; i++ {
		if slotActions[i] == actionDischarge {
			price := predictions[i].price
			// If this discharge is not profitable (price <= profitThreshold), consider removing it
			if price < cheapestDischargePrice && price <= profitThreshold {
				cheapestDischargeIdx = i
				cheapestDischargePrice = price
			}
		}
	}

	if cheapestDischargeIdx >= 0 {
		slotActions[cheapestDischargeIdx] = actionIdle
		log.Debugf("Battery schedule optimizer: reduced unprofitable discharge at slot %d (price %.4f, threshold %.4f)",
			cheapestDischargeIdx, cheapestDischargePrice, profitThreshold)
		return true
	}

	return false
}

// reduceAnyDischarge reduces the cheapest discharge slot as a last resort
func (o *BatteryScheduleOptimizer) reduceAnyDischarge(
	predictions []*slotPrediction,
	slotActions []slotAction,
	beforeIdx int,
) bool {
	var cheapestDischargeIdx = -1
	var cheapestDischargePrice float32 = 999999

	for i := 0; i <= beforeIdx; i++ {
		if slotActions[i] == actionDischarge {
			if predictions[i].price < cheapestDischargePrice {
				cheapestDischargeIdx = i
				cheapestDischargePrice = predictions[i].price
			}
		}
	}

	if cheapestDischargeIdx >= 0 {
		slotActions[cheapestDischargeIdx] = actionIdle
		log.Debugf("Battery schedule optimizer: reduced discharge at slot %d (price %.4f) to avoid MinSoC",
			cheapestDischargeIdx, cheapestDischargePrice)
		return true
	}

	return false
}

// addSurvivalCharging finds the cheapest idle slot before the given index and marks it for charging
// Returns true if a slot was converted to charging, false otherwise
func (o *BatteryScheduleOptimizer) addSurvivalCharging(
	predictions []*slotPrediction,
	slotActions []slotAction,
	beforeIdx int,
	chargeThreshold float32,
) bool {
	// Find all idle or discharge slots before beforeIdx (excluding PV slots)
	type candidate struct {
		index  int
		price  float32
		isIdle bool
	}
	idleCandidates := make([]candidate, 0)
	dischargeCandidates := make([]candidate, 0)

	for i := 0; i <= beforeIdx; i++ {
		if slotActions[i] == actionIdle {
			idleCandidates = append(idleCandidates, candidate{index: i, price: predictions[i].price, isIdle: true})
		} else if slotActions[i] == actionDischarge {
			dischargeCandidates = append(dischargeCandidates, candidate{index: i, price: predictions[i].price, isIdle: false})
		}
	}

	// First priority: convert idle slots to charging (sorted by price, cheapest first)
	if len(idleCandidates) > 0 {
		sort.Slice(idleCandidates, func(i, j int) bool {
			return idleCandidates[i].price < idleCandidates[j].price
		})

		bestIdx := idleCandidates[0].index
		slotActions[bestIdx] = actionChargeGrid
		log.Debugf("Battery schedule optimizer: added survival charging at slot %d (price %.4f)",
			bestIdx, predictions[bestIdx].price)
		return true
	}

	// Second priority: convert discharge slots to charging (most expensive discharge first)
	if len(dischargeCandidates) > 0 {
		sort.Slice(dischargeCandidates, func(i, j int) bool {
			return dischargeCandidates[i].price > dischargeCandidates[j].price
		})

		bestIdx := dischargeCandidates[0].index
		slotActions[bestIdx] = actionChargeGrid // Convert discharge to charge
		log.Debugf("Battery schedule optimizer: converted discharge slot %d to charging for survival (price %.4f)",
			bestIdx, predictions[bestIdx].price)
		return true
	}

	return false
}

// Optimize generates optimal schedule slots using a smarter price-based algorithm
// This considers:
// 1. Energy prices - charge during the CHEAPEST slots, discharge during the MOST EXPENSIVE slots
// 2. PV production - use excess PV to charge battery instead of exporting
// 3. Household consumption - discharge to cover consumption during expensive periods
// 4. Battery constraints - SoC limits, power limits, efficiency
//
// The algorithm uses a two-pass approach:
// Pass 1: Identify which slots should charge/discharge based on price ranking
// Pass 2: Process chronologically to calculate actual SoC evolution
func (o *BatteryScheduleOptimizer) Optimize(currentSoC float32, predictions []*slotPrediction) []*battery.ScheduleSlot {
	slots := make([]*battery.ScheduleSlot, 0)

	if len(predictions) == 0 {
		return slots
	}

	// Create indexed predictions for sorting
	type indexedPrediction struct {
		*slotPrediction
		index int
	}
	indexed := make([]*indexedPrediction, len(predictions))
	for i, p := range predictions {
		indexed[i] = &indexedPrediction{p, i}
	}

	// Sort by price to find cheapest and most expensive slots
	sortedByPrice := make([]*indexedPrediction, len(indexed))
	copy(sortedByPrice, indexed)
	sort.Slice(sortedByPrice, func(i, j int) bool {
		return sortedByPrice[i].price < sortedByPrice[j].price
	})

	// Calculate price thresholds based on percentiles
	chargePercentileIdx := len(sortedByPrice) * 30 / 100
	dischargePercentileIdx := len(sortedByPrice) * 70 / 100
	if chargePercentileIdx >= len(sortedByPrice) {
		chargePercentileIdx = len(sortedByPrice) - 1
	}
	if dischargePercentileIdx >= len(sortedByPrice) {
		dischargePercentileIdx = len(sortedByPrice) - 1
	}

	chargeThreshold := sortedByPrice[chargePercentileIdx].price
	dischargeThreshold := sortedByPrice[dischargePercentileIdx].price

	// Adjust discharge threshold for round-trip efficiency
	efficiencyFactor := 1.0 / o.config.RoundTripEfficiency
	minProfitableDischargePrice := chargeThreshold * float32(efficiencyFactor) * 1.1
	if dischargeThreshold < minProfitableDischargePrice {
		dischargeThreshold = minProfitableDischargePrice
	}

	log.Debugf("Battery schedule optimizer: price thresholds - charge below %.4f, discharge above %.4f",
		chargeThreshold, dischargeThreshold)

	// Estimate total energy needed for charging
	// We want to charge enough to cover consumption during expensive/idle periods
	totalCapacityKwh := o.config.BatteryCapacityKwh * (o.config.MaxSoC - o.config.MinSoC) / 100
	currentEnergyKwh := o.config.BatteryCapacityKwh * (currentSoC - o.config.MinSoC) / 100

	// Estimate consumption during non-cheap periods
	var estimatedConsumptionKwh float32
	for _, p := range predictions {
		if p.price > chargeThreshold && p.netGridDemandW > 0 {
			slotDurationHours := p.endTime.Sub(p.startTime).Hours()
			estimatedConsumptionKwh += p.netGridDemandW / 1000 * float32(slotDurationHours)
		}
	}

	// Energy needed = consumption we need to cover + buffer to reach max SoC
	energyNeededKwh := totalCapacityKwh - currentEnergyKwh + estimatedConsumptionKwh

	// Initialize all slots as idle
	slotActions := make([]slotAction, len(predictions))

	// Pass 1a: Mark PV charging slots (always beneficial - free energy)
	for _, ip := range indexed {
		if ip.netGridDemandW < 0 {
			slotActions[ip.index] = actionChargePV
		}
	}

	// Pass 1b: Mark the CHEAPEST slots for grid charging
	// Only charge in enough slots to cover our energy needs
	var allocatedChargeEnergyKwh float32
	for _, ip := range sortedByPrice {
		if slotActions[ip.index] != actionIdle {
			continue // Already marked for PV charging
		}
		if ip.price > chargeThreshold {
			break // Prices are sorted, so all remaining are more expensive
		}
		if allocatedChargeEnergyKwh >= energyNeededKwh {
			break // We have enough charging slots allocated
		}

		slotDurationHours := ip.endTime.Sub(ip.startTime).Hours()
		maxChargeEnergyKwh := o.config.MaxChargePowerW / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		slotActions[ip.index] = actionChargeGrid
		allocatedChargeEnergyKwh += maxChargeEnergyKwh
	}

	// Pass 1c: Mark the MOST EXPENSIVE slots for discharging
	// Sort by price descending for discharge allocation
	sort.Slice(sortedByPrice, func(i, j int) bool {
		return sortedByPrice[i].price > sortedByPrice[j].price
	})

	for _, ip := range sortedByPrice {
		if slotActions[ip.index] != actionIdle {
			continue // Already marked for charging
		}
		if ip.price < dischargeThreshold {
			break // Prices are sorted descending, so all remaining are cheaper
		}
		slotActions[ip.index] = actionDischarge
	}

	// Pass 1d: SURVIVAL CHECK - ensure battery doesn't hit MinSoC before reaching charging slots
	// Simulate forward and add "survival charging" at the cheapest available slots when needed
	o.ensureBatterySurvival(predictions, slotActions, currentSoC, chargeThreshold)

	log.Debugf("Battery schedule optimizer: allocated %.2f kWh of charging capacity, needed %.2f kWh",
		allocatedChargeEnergyKwh, energyNeededKwh)

	// Pass 2: Process chronologically to calculate actual power and SoC
	simulatedSoC := currentSoC

	for i, p := range predictions {
		slotDurationHours := p.endTime.Sub(p.startTime).Hours()
		action := slotActions[i]

		// Calculate household consumption drain for this slot
		var householdEnergyDrainKwh float32
		if p.netGridDemandW > 0 {
			householdEnergyDrainKwh = p.netGridDemandW / 1000 * float32(slotDurationHours)
		}

		var chargePower float32
		var chargingSource battery.ChargingSource
		var energyChangeKwh float32

		switch action {
		case actionChargePV:
			// Charge from excess PV
			excessPvW := -p.netGridDemandW
			availableCapacityKwh := (o.config.MaxSoC - simulatedSoC) / 100 * o.config.BatteryCapacityKwh
			if availableCapacityKwh > 0 && excessPvW > 0 {
				maxChargePower := minFloat32(excessPvW, o.config.MaxChargePowerW)
				maxChargeEnergyKwh := maxChargePower / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
				chargeEnergyKwh := minFloat32(maxChargeEnergyKwh, availableCapacityKwh)
				chargePower = chargeEnergyKwh / float32(slotDurationHours) * 1000 / o.config.RoundTripEfficiency
				chargingSource = battery.ChargingSourcePV
				energyChangeKwh = chargeEnergyKwh
			}

		case actionChargeGrid:
			// Charge from grid
			availableCapacityKwh := (o.config.MaxSoC - simulatedSoC) / 100 * o.config.BatteryCapacityKwh
			if availableCapacityKwh > 0 {
				maxChargeEnergyKwh := o.config.MaxChargePowerW / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
				chargeEnergyKwh := minFloat32(maxChargeEnergyKwh, availableCapacityKwh)
				chargePower = chargeEnergyKwh / float32(slotDurationHours) * 1000 / o.config.RoundTripEfficiency
				chargingSource = battery.ChargingSourceGrid
				energyChangeKwh = chargeEnergyKwh - householdEnergyDrainKwh
			} else {
				energyChangeKwh = -householdEnergyDrainKwh
			}

		case actionDischarge:
			// Discharge for price arbitrage or to cover consumption
			availableEnergyKwh := (simulatedSoC - o.config.MinSoC) / 100 * o.config.BatteryCapacityKwh
			if availableEnergyKwh > 0 {
				var targetDischargePower float32
				if p.netGridDemandW > 0 {
					targetDischargePower = minFloat32(p.netGridDemandW, o.config.MaxDischargePowerW)
				} else {
					targetDischargePower = o.config.MaxDischargePowerW
				}
				maxDischargeEnergyKwh := targetDischargePower / 1000 * float32(slotDurationHours)
				dischargeEnergyKwh := minFloat32(maxDischargeEnergyKwh, availableEnergyKwh)
				chargePower = -dischargeEnergyKwh / float32(slotDurationHours) * 1000
				chargingSource = battery.ChargingSourceNone
				energyChangeKwh = -dischargeEnergyKwh
			} else {
				energyChangeKwh = -householdEnergyDrainKwh
			}

		default: // actionIdle
			energyChangeKwh = -householdEnergyDrainKwh
		}

		// Update simulated SoC
		simulatedSoC += energyChangeKwh / o.config.BatteryCapacityKwh * 100
		simulatedSoC = maxFloat32(o.config.MinSoC, minFloat32(o.config.MaxSoC, simulatedSoC))

		slot := battery.NewScheduleSlot(p.startTime, p.endTime).
			SetChargePower(chargePower).
			SetChargingSource(chargingSource).
			SetPredictedSoC(simulatedSoC).
			SetPricePerKwh(p.price)

		slots = append(slots, slot)
	}

	return slots
}

// OptimizeWithPrices is a convenience method for testing with just prices
func (o *BatteryScheduleOptimizer) OptimizeWithPrices(now time.Time, energyPrices []*prices.EnergyPrice) []*battery.ScheduleSlot {
	predictions := make([]*slotPrediction, 0)
	for _, ep := range energyPrices {
		if ep.EndTime.Before(now) || ep.EndTime.Equal(now) {
			continue
		}
		startTime := ep.Time
		if startTime.Before(now) {
			startTime = now
		}
		predictions = append(predictions, &slotPrediction{
			startTime:             startTime,
			endTime:               ep.EndTime,
			price:                 ep.ConsumptionPrice,
			predictedPvPowerW:     0,
			predictedConsumptionW: 0,
			netGridDemandW:        0,
		})
	}
	return o.Optimize(o.config.MinSoC+((o.config.MaxSoC-o.config.MinSoC)/2), predictions)
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

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
	// HouseholdSourceName is the name of the household consumption source (grid name)
	HouseholdSourceName string
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
		config.HouseholdSourceName = system.Grid().Name()
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
// Excludes AC loads that have percentageFromGrid=100 (user-managed loads like EV chargers)
func (o *BatteryScheduleOptimizer) getPredictedConsumption(slotStart, slotEnd time.Time) float32 {
	if o.repository == nil || o.config.HouseholdSourceName == "" {
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
		states, err := o.repository.ElectricityStates(
			historicalStart,
			historicalEnd,
			o.config.HouseholdSourceName,
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
				// Grid consumption is positive when consuming
				power := avg.TotalPower()
				if power > 0 {
					// Subtract the portion of AC load consumption not managed by optimizer
					// Factor = 1 - (percentageFromGrid / 100)
					// We subtract power * (1 - factor) = power * percentageFromGrid / 100
					excludedPower := o.getAcLoadExcludedPower(historicalStart, historicalEnd)
					power -= excludedPower
					if power < 0 {
						power = 0
					}
					sumPower += power
					count++
				}
			}
		}
	}

	if count > 0 {
		return sumPower / float32(count)
	}
	return 0
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

// Optimize generates optimal schedule slots using a greedy price-based algorithm
// This considers:
// 1. Energy prices - charge when cheap, discharge when expensive
// 2. PV production - use excess PV to charge battery instead of exporting
// 3. Household consumption - discharge to cover consumption during expensive periods
// 4. Battery constraints - SoC limits, power limits, efficiency
func (o *BatteryScheduleOptimizer) Optimize(currentSoC float32, predictions []*slotPrediction) []*battery.ScheduleSlot {
	slots := make([]*battery.ScheduleSlot, 0)

	if len(predictions) == 0 {
		return slots
	}

	// Sort prices to determine percentile-based thresholds
	pricesSorted := make([]float32, len(predictions))
	for i, p := range predictions {
		pricesSorted[i] = p.price
	}
	sort.Slice(pricesSorted, func(i, j int) bool {
		return pricesSorted[i] < pricesSorted[j]
	})

	// Use percentile-based thresholds:
	// - Charge threshold: 30th percentile (charge during cheapest 30% of hours)
	// - Discharge threshold: 70th percentile (discharge during most expensive 30% of hours)
	chargePercentileIdx := len(pricesSorted) * 30 / 100
	dischargePercentileIdx := len(pricesSorted) * 70 / 100

	// Ensure indices are within bounds
	if chargePercentileIdx >= len(pricesSorted) {
		chargePercentileIdx = len(pricesSorted) - 1
	}
	if dischargePercentileIdx >= len(pricesSorted) {
		dischargePercentileIdx = len(pricesSorted) - 1
	}

	chargeThreshold := pricesSorted[chargePercentileIdx]
	dischargeThreshold := pricesSorted[dischargePercentileIdx]

	// Adjust discharge threshold for round-trip efficiency
	// Only discharge if we can make a profit after accounting for losses
	efficiencyFactor := 1.0 / o.config.RoundTripEfficiency
	minProfitableDischargePrice := chargeThreshold * float32(efficiencyFactor) * 1.1
	if dischargeThreshold < minProfitableDischargePrice {
		dischargeThreshold = minProfitableDischargePrice
	}

	log.Debugf("Battery schedule optimizer: price thresholds - charge below %.4f, discharge above %.4f",
		chargeThreshold, dischargeThreshold)

	// Create indexed predictions for sorting
	type indexedPrediction struct {
		*slotPrediction
		index int
	}
	indexed := make([]*indexedPrediction, len(predictions))
	for i, p := range predictions {
		indexed[i] = &indexedPrediction{p, i}
	}

	// Sort by effective cost (price adjusted by net demand)
	// Slots with excess PV (negative net demand) are more attractive for charging
	// Slots with high consumption are more attractive for discharging
	sortedByValue := make([]*indexedPrediction, len(indexed))
	copy(sortedByValue, indexed)
	sort.Slice(sortedByValue, func(i, j int) bool {
		// Effective charge cost = price - value of storing excess PV
		// Lower is better for charging
		costI := sortedByValue[i].price
		costJ := sortedByValue[j].price
		// If there's excess PV, it's effectively free energy to store
		if sortedByValue[i].netGridDemandW < 0 {
			costI = 0 // Excess PV makes charging very attractive
		}
		if sortedByValue[j].netGridDemandW < 0 {
			costJ = 0
		}
		return costI < costJ
	})

	// slotDecision holds both power and source for a slot
	type slotDecision struct {
		power  float32
		source battery.ChargingSource
	}

	// Decisions map: index -> decision (power + source)
	slotDecisions := make(map[int]slotDecision)
	simulatedSoC := currentSoC

	// First pass: Handle excess PV - always store it if possible
	for _, ip := range indexed {
		if ip.netGridDemandW >= 0 {
			continue // No excess PV
		}

		excessPvW := -ip.netGridDemandW
		availableCapacity := (o.config.MaxSoC - simulatedSoC) / 100 * o.config.BatteryCapacityKwh
		if availableCapacity <= 0 {
			continue
		}

		// Charge from excess PV (limited by PV output and battery capacity)
		chargePower := minFloat32(excessPvW, o.config.MaxChargePowerW)
		slotDurationHours := ip.endTime.Sub(ip.startTime).Hours()
		chargeEnergy := chargePower / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		chargeEnergy = minFloat32(chargeEnergy, availableCapacity)
		chargePower = chargeEnergy / float32(slotDurationHours) * 1000 / o.config.RoundTripEfficiency

		if chargePower > 0 {
			slotDecisions[ip.index] = slotDecision{power: chargePower, source: battery.ChargingSourcePV}
			simulatedSoC += chargeEnergy / o.config.BatteryCapacityKwh * 100
		}
	}

	// Second pass: Charge from grid during cheap periods
	for _, ip := range sortedByValue {
		if _, exists := slotDecisions[ip.index]; exists {
			continue // Already decided (PV charging)
		}
		if ip.price > chargeThreshold {
			continue // Too expensive
		}

		availableCapacity := (o.config.MaxSoC - simulatedSoC) / 100 * o.config.BatteryCapacityKwh
		if availableCapacity <= 0 {
			continue
		}

		slotDurationHours := ip.endTime.Sub(ip.startTime).Hours()
		maxChargeEnergy := o.config.MaxChargePowerW / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		chargeEnergy := minFloat32(availableCapacity, maxChargeEnergy)
		chargePower := chargeEnergy / float32(slotDurationHours) * 1000 / o.config.RoundTripEfficiency

		slotDecisions[ip.index] = slotDecision{power: chargePower, source: battery.ChargingSourceGrid}
		simulatedSoC += chargeEnergy / o.config.BatteryCapacityKwh * 100
	}

	// Third pass: Discharge during expensive periods
	// Sort by discharge value (high price first, then by consumption if available)
	sort.Slice(sortedByValue, func(i, j int) bool {
		// Value of discharging = price (with bonus for consumption coverage)
		valueI := sortedByValue[i].price
		valueJ := sortedByValue[j].price
		// Add bonus for slots with consumption to offset
		if sortedByValue[i].netGridDemandW > 0 {
			valueI *= 1.5 // Prefer discharging when there's consumption to offset
		}
		if sortedByValue[j].netGridDemandW > 0 {
			valueJ *= 1.5
		}
		return valueI > valueJ // Higher value first
	})

	for _, ip := range sortedByValue {
		if _, exists := slotDecisions[ip.index]; exists {
			continue // Already decided
		}
		if ip.price < dischargeThreshold {
			continue // Price too low to justify discharge
		}
		if ip.netGridDemandW < 0 {
			continue // Excess PV - don't discharge during solar production
		}

		availableEnergy := (simulatedSoC - o.config.MinSoC) / 100 * o.config.BatteryCapacityKwh
		if availableEnergy <= 0 {
			continue
		}

		slotDurationHours := ip.endTime.Sub(ip.startTime).Hours()
		// Discharge power: cover consumption if known, otherwise use max power for arbitrage
		var targetDischargePower float32
		if ip.netGridDemandW > 0 {
			targetDischargePower = minFloat32(ip.netGridDemandW, o.config.MaxDischargePowerW)
		} else {
			targetDischargePower = o.config.MaxDischargePowerW // Pure price arbitrage
		}
		maxDischargeEnergy := targetDischargePower / 1000 * float32(slotDurationHours)
		dischargeEnergy := minFloat32(availableEnergy, maxDischargeEnergy)
		dischargePower := -dischargeEnergy / float32(slotDurationHours) * 1000 // Negative for discharge

		slotDecisions[ip.index] = slotDecision{power: dischargePower, source: battery.ChargingSourceNone}
		simulatedSoC -= dischargeEnergy / o.config.BatteryCapacityKwh * 100
	}

	// Build final schedule slots in chronological order
	simulatedSoC = currentSoC
	for i, p := range predictions {
		decision, hasDecision := slotDecisions[i]
		chargePower := float32(0)
		chargingSource := battery.ChargingSourceNone
		if hasDecision {
			chargePower = decision.power
			chargingSource = decision.source
		}

		// Calculate predicted SoC at end of slot
		slotDurationHours := p.endTime.Sub(p.startTime).Hours()
		var energyChange float32
		if chargePower > 0 {
			energyChange = chargePower / 1000 * float32(slotDurationHours) * o.config.RoundTripEfficiency
		} else {
			energyChange = chargePower / 1000 * float32(slotDurationHours)
		}
		simulatedSoC += energyChange / o.config.BatteryCapacityKwh * 100
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

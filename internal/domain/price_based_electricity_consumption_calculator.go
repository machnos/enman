package domain

import (
	"context"
	"enman/internal/domain/mpc"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"sync"
	"sync/atomic"
	"time"
)

// reoptimizationInterval defines how often the MPC optimization runs
const reoptimizationInterval = 5 * time.Minute

type PriceBasedElectricityConsumptionCalculator struct {
	repository          repository.Repository
	providerName        string
	batteries           []*Battery
	pvNames             []string
	roundTripEfficiency float32
	mpcOptimizer        *mpc.MPCOptimizer
	dataFetcher         *mpc.DataFetcher

	// Cached optimization result (accessed atomically)
	cachedResult atomic.Pointer[mpc.OptimizationResult]

	// Last available charge power for optimization
	lastAvailableChargePower atomic.Value // float32

	// Background optimization control
	optimizationTicker *time.Ticker
	stopChan           chan struct{}
	stopOnce           sync.Once
	running            atomic.Bool
}

func NewPriceBasedElectricityConsumptionCalculator(
	repository repository.Repository,
	providerName string,
	batteries []*Battery,
	pvNames []string,
	roundTripEfficiency float32,
) *PriceBasedElectricityConsumptionCalculator {
	// Default to 90% if not configured
	if roundTripEfficiency <= 0 || roundTripEfficiency > 100 {
		roundTripEfficiency = 90.0
	}
	calc := &PriceBasedElectricityConsumptionCalculator{
		repository:          repository,
		providerName:        providerName,
		batteries:           batteries,
		pvNames:             pvNames,
		roundTripEfficiency: roundTripEfficiency,
		mpcOptimizer:        mpc.NewMPCOptimizer(15.0, 95.0), // Min 15%, Max 95% SoC
		dataFetcher:         mpc.NewDataFetcher(repository),
		stopChan:            make(chan struct{}),
	}
	calc.lastAvailableChargePower.Store(float32(0))
	return calc
}

// Start begins the background optimization goroutine
func (p *PriceBasedElectricityConsumptionCalculator) Start(ctx context.Context) {
	if p.running.Swap(true) {
		// Already running
		return
	}

	// Run initial optimization immediately
	go p.runOptimizationAsync()

	// Start periodic optimization
	p.optimizationTicker = time.NewTicker(reoptimizationInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.Stop()
				return
			case <-p.stopChan:
				return
			case <-p.optimizationTicker.C:
				p.runOptimizationAsync()
			}
		}
	}()
}

// Stop stops the background optimization goroutine
func (p *PriceBasedElectricityConsumptionCalculator) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
		if p.optimizationTicker != nil {
			p.optimizationTicker.Stop()
		}
		p.running.Store(false)
	})
}

// CalculateAddition returns the optimal power to pull from the grid for battery charging.
// This method is designed to be called frequently (multiple times per second) and is very fast
// as it only reads from a cached result that is updated by a background goroutine.
func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) float32 {
	// Store the latest available charge power for the next optimization run
	p.lastAvailableChargePower.Store(availableChargePower)

	// Get cached result (lock-free read)
	result := p.cachedResult.Load()
	if result == nil || !result.Success {
		return 0.0
	}

	optimalPower := result.OptimalAction

	// Ensure we don't exceed available power
	if optimalPower > 0 && optimalPower > availableChargePower {
		optimalPower = availableChargePower
	}

	return optimalPower
}

// runOptimizationAsync runs the optimization in the background and updates the cached result
func (p *PriceBasedElectricityConsumptionCalculator) runOptimizationAsync() {
	log.Debug("Running MPC optimization for battery charging schedule")

	availableChargePower := p.lastAvailableChargePower.Load().(float32)
	result := p.runOptimization(time.Now(), availableChargePower)

	if result.Success {
		p.cachedResult.Store(result)
	} else {
		log.Warningf("MPC optimization failed: %s", result.Message)
	}
}

func (p *PriceBasedElectricityConsumptionCalculator) runOptimization(
	currentTime time.Time,
	availableChargePower float32,
) *mpc.OptimizationResult {
	// Get primary battery (use first battery for now)
	if len(p.batteries) == 0 {
		return &mpc.OptimizationResult{
			Success: false,
			Message: "No batteries configured",
		}
	}

	battery := p.batteries[0]
	if !battery.IsMeasurementStarted() {
		return &mpc.OptimizationResult{
			Success: false,
			Message: "Battery measurement not started",
		}
	}

	// Fetch current battery SoC
	currentSoC := battery.State().SoC()

	// Fetch energy prices for next 24 hours
	priceFrom := currentTime.Truncate(time.Hour)
	priceTill := priceFrom.Add(24 * time.Hour)

	energyPrices, err := p.repository.EnergyPrices(priceFrom, priceTill, p.providerName, prices.EnergyTypeElectricity)
	if err != nil {
		log.Warningf("Failed to fetch energy prices: %v", err)
		return &mpc.OptimizationResult{
			Success: false,
			Message: "Failed to fetch energy prices",
		}
	}

	// Build price forecast
	priceForecast := mpc.BuildPriceForecast(currentTime, energyPrices)

	// Fetch historical consumption data (last 7 days)
	gridConsumption, err := p.dataFetcher.FetchHistoricalGridConsumption(p.providerName, 7)
	if err != nil {
		log.Warningf("Failed to fetch historical consumption: %v", err)
	}

	// Build consumption forecast
	consumptionForecast := buildForecastFromHistorical(gridConsumption.HourlyAverages[:], currentTime.Hour())

	// Fetch historical PV production
	pvProduction, err := p.dataFetcher.FetchHistoricalPVProduction(p.pvNames, 7)
	if err != nil {
		log.Debugf("Using default PV production pattern: %v", err)
	}

	// Build PV forecast
	pvForecast := buildForecastFromHistorical(pvProduction.HourlyAverages[:], currentTime.Hour())

	// Get battery round-trip efficiency from configuration
	roundTripEff := p.roundTripEfficiency

	// Determine max charge power - use the minimum of battery capability and available grid power
	maxChargePower := battery.MaxChargePower()
	if availableChargePower > 0 && availableChargePower < maxChargePower {
		maxChargePower = availableChargePower
	}

	// Create optimization input
	input := &mpc.OptimizationInput{
		CurrentSoC:             currentSoC,
		CurrentTime:            currentTime,
		BatteryRoundTripEff:    roundTripEff,
		BatteryCapacity:        float32(battery.Capacity()) * battery.Voltage() / 1000.0, // Convert Ah*V to kWh
		MaxChargePower:         maxChargePower,
		MaxDischargePower:      battery.MaxDischargePower(),
		MinSoC:                 15.0,
		MaxSoC:                 95.0,
		EnergyPrices:           priceForecast,
		ForecastedConsumption:  consumptionForecast,
		ForecastedPVProduction: pvForecast,
	}

	// Run MPC optimization
	result := p.mpcOptimizer.Optimize(input)

	if result.Success {
		log.Infof("MPC optimization successful: Total cost: %.2f €, Recommended power: %.0f W",
			result.TotalCost, result.OptimalAction)
		log.Debugf("SoC trajectory: Start=%.1f%%, End=%.1f%%",
			result.SoCTrajectory[0], result.SoCTrajectory[len(result.SoCTrajectory)-1])
	}

	return result
}

// buildForecastFromHistorical creates a 24-hour forecast from hourly historical data
func buildForecastFromHistorical(historicalHourly []float32, currentHour int) []float32 {
	forecast := make([]float32, mpc.NumTimeSteps)

	for i := 0; i < mpc.NumTimeSteps; i++ {
		hourIndex := (currentHour + i) % 24
		if hourIndex < len(historicalHourly) {
			forecast[i] = historicalHourly[hourIndex]
		} else {
			forecast[i] = 0.0
		}
	}

	return forecast
}

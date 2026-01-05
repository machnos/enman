package domain

import (
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"fmt"
	"math"
	"time"
)

// ChargingDip represents a low-price period and the peak it can serve
type ChargingDip struct {
	StartIndex  int     // Index in the energyPrices array
	EndIndex    int     // Index in the energyPrices array
	PeakIndex   int     // Index of the peak this dip charges for
	Price       float32 // Average price during this dip
	IsProcessed bool    // Whether we've already charged in this dip today
}

type PriceBasedElectricityConsumptionCalculator struct {
	repository                    repository.EnergyPrice
	providerName                  string
	batteries                     []*Battery
	enabled                       bool
	peakDetectionStdDevMultiplier float32
	socThreshold                  float32
	cachedEnergyPrices            []*prices.EnergyPrice
	cachedPricesTime              time.Time
	cacheTTL                      time.Duration
	lastAnalyzedPricesTime        time.Time     // Track when we last analyzed the prices
	lastAnalyzedPricesStart       time.Time     // Track the start time of the last analyzed price window
	lastAnalyzedPricesEnd         time.Time     // Track the end time of the last analyzed price window
	chargingDips                  []ChargingDip // Dips for the current 48-hour window
}

// NewPriceBasedElectricityConsumptionCalculator creates a new calculator for determining
// battery charging power based on energy prices.
// peakDetectionStdDevMultiplier controls peak detection sensitivity (default 1.0):
//
//	0.5 = conservative (detects more peaks)
//	1.0 = optimal (balanced)
//	1.5 = aggressive (only extreme peaks)
//
// socThreshold is the SoC percentage below which survival charging is used (default 25.0)
func NewPriceBasedElectricityConsumptionCalculator(
	repository repository.EnergyPrice,
	providerName string,
	batteries []*Battery,
	peakDetectionStdDevMultiplier float32,
	socThreshold float32,
) *PriceBasedElectricityConsumptionCalculator {
	if peakDetectionStdDevMultiplier <= 0 {
		peakDetectionStdDevMultiplier = 1.0
	}
	if socThreshold < 0 || socThreshold > 100 {
		socThreshold = 25.0
	}

	enabled := repository != nil && providerName != "" && len(batteries) > 0
	if !enabled {
		log.Warningf("PriceBasedElectricityConsumptionCalculator disabled: repository=%v, providerName=%s, batteries=%d",
			repository != nil, providerName, len(batteries))
	}
	return &PriceBasedElectricityConsumptionCalculator{
		repository:                    repository,
		providerName:                  providerName,
		batteries:                     batteries,
		enabled:                       enabled,
		peakDetectionStdDevMultiplier: peakDetectionStdDevMultiplier,
		socThreshold:                  socThreshold,
		cacheTTL:                      1 * time.Hour,
		lastAnalyzedPricesTime:        time.Time{},
		lastAnalyzedPricesStart:       time.Time{},
		lastAnalyzedPricesEnd:         time.Time{},
		chargingDips:                  []ChargingDip{},
	}
}

// CalculateAddition determines how much power should be allocated to battery charging
// based on daily analysis of price dips and peaks.
// Once per day, it identifies optimal charging dips before price peaks.
// At each moment, it decides whether to charge based on current dip and SoC status.
// Energy prices are cached for 1 hour to avoid redundant API calls.
func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) int {
	if !p.enabled {
		return 0
	}

	if availableChargePower <= 0 {
		log.Debugf("No available charge power for battery charging (available: %f)", availableChargePower)
		return 0
	}

	now := time.Now()
	endTime := now.Add(48 * time.Hour)

	// Check if cached prices are still valid
	var energyPrices []*prices.EnergyPrice
	if p.cachedEnergyPrices != nil && now.Before(p.cachedPricesTime.Add(p.cacheTTL)) {
		log.Debugf("Using cached energy prices (cached at %v, TTL: %v)", p.cachedPricesTime, p.cacheTTL)
		energyPrices = p.cachedEnergyPrices
	} else {
		// Cache expired or not yet set, fetch new prices
		var err error
		energyPrices, err = p.repository.EnergyPrices(now, endTime, p.providerName, prices.EnergyTypeElectricity)
		if err != nil {
			log.Warningf("Failed to retrieve energy prices for price-based charging: %v", err)
			return 0
		}

		// Update cache
		p.cachedEnergyPrices = energyPrices
		p.cachedPricesTime = now
		log.Debugf("Fetched and cached %d energy prices", len(energyPrices))
	}

	if len(energyPrices) < 1 {
		log.Debugf("Insufficient price data for battery charging decision (got %d prices)", len(energyPrices))
		return 0
	}

	// Check if we have truly new prices (start or end of window changed)
	// This happens once per day when new prices arrive, not on every cache refresh
	var pricesChanged bool
	if len(energyPrices) > 0 {
		currentWindowStart := energyPrices[0].Time
		currentWindowEnd := energyPrices[len(energyPrices)-1].EndTime

		if p.lastAnalyzedPricesStart != currentWindowStart || p.lastAnalyzedPricesEnd != currentWindowEnd {
			pricesChanged = true
			log.Infof("New prices detected: price window changed from [%v, %v] to [%v, %v]",
				p.lastAnalyzedPricesStart, p.lastAnalyzedPricesEnd, currentWindowStart, currentWindowEnd)
		}
	}

	// Analyze dips and peaks only if prices have actually changed (new timeslots)
	if pricesChanged {
		log.Infof("New prices retrieved, analyzing dips and peaks for 48-hour window")
		p.analyzeChargingDips(energyPrices)
		p.lastAnalyzedPricesTime = now
		if len(energyPrices) > 0 {
			p.lastAnalyzedPricesStart = energyPrices[0].Time
			p.lastAnalyzedPricesEnd = energyPrices[len(energyPrices)-1].EndTime
		}
	}

	// Find the current price period
	var currentPrice *prices.EnergyPrice
	currentPriceIndex := -1
	for i := range energyPrices {
		if energyPrices[i].Time.Before(now) && energyPrices[i].EndTime.After(now) {
			currentPrice = energyPrices[i]
			currentPriceIndex = i
			break
		}
	}

	if currentPrice == nil {
		log.Debugf("Current time is not within any price period")
		return 0
	}

	// Check if we're currently in a charging dip
	var currentDip *ChargingDip
	for i := range p.chargingDips {
		if currentPriceIndex >= p.chargingDips[i].StartIndex && currentPriceIndex <= p.chargingDips[i].EndIndex {
			currentDip = &p.chargingDips[i]
			break
		}
	}

	if currentDip == nil {
		log.Debugf("Current time is not in any identified charging dip")
		return 0
	}

	// Determine if we should charge in this dip
	shouldCharge := false
	reason := ""

	// Calculate average SoC
	totalSoC := float32(0)
	for _, battery := range p.batteries {
		totalSoC += battery.State().SoC()
	}
	avgSoC := totalSoC / float32(len(p.batteries))

	// Check peak that this dip serves
	peakPrice := float32(0)
	if currentDip.PeakIndex < len(energyPrices) {
		peakPrice = energyPrices[currentDip.PeakIndex].ConsumptionPrice
	}

	// Strategy 1: Survival charging - if SoC is too low for the upcoming peak
	if avgSoC < p.socThreshold {
		shouldCharge = true
		reason = "Survival charging: SoC below threshold"
	} else {
		// Strategy 2: Normal charging - if we haven't charged this dip yet and it's economically viable
		if !currentDip.IsProcessed {
			roundTripEfficiency := getAverageRoundTripEfficiency(p.batteries)
			if isEconomicallyViable(currentDip.Price, peakPrice, roundTripEfficiency) {
				shouldCharge = true
				reason = "Normal charging: Economically viable dip"
				currentDip.IsProcessed = true
			} else {
				reason = fmt.Sprintf("Dip not economically viable (dip: €%.4f, peak: €%.4f)", currentDip.Price, peakPrice)
			}
		} else {
			reason = "Dip already processed today"
		}
	}

	if !shouldCharge {
		log.Debugf("Not charging: %s", reason)
		return 0
	}

	log.Infof("Charging battery in dip: %s (price: €%.4f, peak: €%.4f, SoC: %.2f%%)",
		reason, currentDip.Price, peakPrice, avgSoC)

	// Calculate how much power each battery can accept
	totalAddition := 0
	remainingChargePower := availableChargePower

	for _, battery := range p.batteries {
		if remainingChargePower <= 0 {
			break
		}

		chargeDuration, batteryChargePower := battery.ChargeDuration(remainingChargePower, 100)
		if batteryChargePower <= 0 {
			continue
		}

		powerToAdd := int(batteryChargePower)
		totalAddition += powerToAdd
		remainingChargePower -= batteryChargePower

		log.Debugf("Adding %d W for battery charging (duration: %v, current price: €%.4f)",
			powerToAdd, chargeDuration, currentPrice.ConsumptionPrice)
	}

	return totalAddition
}

// analyzeChargingDips identifies price dips and their corresponding peaks in the 48-hour window
func (p *PriceBasedElectricityConsumptionCalculator) analyzeChargingDips(energyPrices []*prices.EnergyPrice) {
	p.chargingDips = []ChargingDip{}

	if len(energyPrices) < 2 {
		log.Debugf("Not enough price data for dip analysis")
		return
	}

	// Calculate price statistics
	totalPrice := float32(0)
	maxPrice := energyPrices[0].ConsumptionPrice
	minPrice := energyPrices[0].ConsumptionPrice

	for _, ep := range energyPrices {
		totalPrice += ep.ConsumptionPrice
		if ep.ConsumptionPrice > maxPrice {
			maxPrice = ep.ConsumptionPrice
		}
		if ep.ConsumptionPrice < minPrice {
			minPrice = ep.ConsumptionPrice
		}
	}

	avgPrice := totalPrice / float32(len(energyPrices))

	// Calculate standard deviation
	varianceSum := float32(0)
	for _, ep := range energyPrices {
		diff := ep.ConsumptionPrice - avgPrice
		varianceSum += diff * diff
	}
	stdDev := float32(math.Sqrt(float64(varianceSum / float32(len(energyPrices)))))

	log.Debugf("Price statistics (48-hour window): min=€%.4f, avg=€%.4f, max=€%.4f, stdDev=€%.4f",
		minPrice, avgPrice, maxPrice, stdDev)

	// Detect peaks using configurable threshold
	peakThreshold := avgPrice + (p.peakDetectionStdDevMultiplier * stdDev)
	dipThreshold := avgPrice - (p.peakDetectionStdDevMultiplier * stdDev)

	log.Debugf("Peak threshold: €%.4f, Dip threshold: €%.4f", peakThreshold, dipThreshold)

	// Find peaks and dips
	peakPeriods := findPeakPeriods(energyPrices, peakThreshold)

	log.Debugf("Found %d peak periods", len(peakPeriods))
	for i, period := range peakPeriods {
		if len(period) > 0 {
			log.Debugf("Peak block %d: indices %d-%d, price range €%.4f-€%.4f",
				i+1, period[0], period[len(period)-1],
				energyPrices[period[0]].ConsumptionPrice,
				energyPrices[period[len(period)-1]].ConsumptionPrice)
		}
	}

	// For each peak, find the preceding dip
	for peakIdx, peakPeriod := range peakPeriods {
		if len(peakPeriod) == 0 {
			continue
		}

		peakStartIndex := peakPeriod[0]

		// Look backward from the peak for the best dip
		bestDipStart := -1
		bestDipEnd := -1
		lowestDipPrice := float32(math.MaxFloat32)

		for i := peakStartIndex - 1; i >= 0; i-- {
			// Stop at previous peak
			if peakIdx > 0 && i <= peakPeriods[peakIdx-1][len(peakPeriods[peakIdx-1])-1] {
				break
			}

			if energyPrices[i].ConsumptionPrice < dipThreshold {
				// This is a dip region
				if bestDipEnd == -1 {
					bestDipEnd = i
				}
				bestDipStart = i

				if energyPrices[i].ConsumptionPrice < lowestDipPrice {
					lowestDipPrice = energyPrices[i].ConsumptionPrice
				}
			}
		}

		if bestDipStart != -1 && bestDipEnd != -1 {
			// Calculate average price in the dip
			dipPriceSum := float32(0)
			for i := bestDipStart; i <= bestDipEnd; i++ {
				dipPriceSum += energyPrices[i].ConsumptionPrice
			}
			avgDipPrice := dipPriceSum / float32(bestDipEnd-bestDipStart+1)

			dip := ChargingDip{
				StartIndex:  bestDipStart,
				EndIndex:    bestDipEnd,
				PeakIndex:   peakStartIndex,
				Price:       avgDipPrice,
				IsProcessed: false,
			}
			p.chargingDips = append(p.chargingDips, dip)

			log.Infof("Identified dip for peak: dip indices %d-%d (avg price €%.4f), peak index %d (price €%.4f)",
				bestDipStart, bestDipEnd, avgDipPrice, peakStartIndex, energyPrices[peakStartIndex].ConsumptionPrice)
		}
	}
}

// findPeakPeriods identifies consecutive periods where prices are above the threshold.
// It groups consecutive expensive periods together as a single "peak period block".
func findPeakPeriods(energyPrices []*prices.EnergyPrice, threshold float32) [][]int {
	var peakPeriods [][]int
	var currentPeak []int

	for i := range energyPrices {
		if energyPrices[i].ConsumptionPrice >= threshold {
			// This is a peak period
			currentPeak = append(currentPeak, i)
		} else {
			// Price dropped below threshold
			if len(currentPeak) > 0 {
				peakPeriods = append(peakPeriods, currentPeak)
				currentPeak = []int{}
			}
		}
	}

	// Don't forget the last peak if it goes to the end
	if len(currentPeak) > 0 {
		peakPeriods = append(peakPeriods, currentPeak)
	}

	return peakPeriods
}

// getAverageRoundTripEfficiency calculates the average round-trip efficiency from all batteries
func getAverageRoundTripEfficiency(batteries []*Battery) float32 {
	if len(batteries) == 0 {
		return 100 // Default to 100% if no batteries
	}
	totalEfficiency := float32(0)
	for _, battery := range batteries {
		totalEfficiency += battery.RoundTripEfficiency()
	}
	return totalEfficiency / float32(len(batteries))
}

// isEconomicallyViable checks if charging during cheapest period is more economical than peak period
// Formula: cheapestPrice * (100 / roundTripEfficiency) < peakPrice
// Example: cheapest=25 cents, peak=27 cents, efficiency=75%
//
//	(25 * 100/75) = 33.33 cents > 27 cents, so NOT viable
//
// Returns true if charging is economically justified
func isEconomicallyViable(cheapestPrice float32, peakPrice float32, roundTripEfficiency float32) bool {
	if roundTripEfficiency <= 0 || roundTripEfficiency > 100 {
		// Invalid efficiency, reject charging to be safe
		return false
	}
	effectiveCost := cheapestPrice * (100 / roundTripEfficiency)
	return effectiveCost < peakPrice
}

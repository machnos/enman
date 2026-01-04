package domain

import (
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"math"
	"time"
)

type PriceBasedElectricityConsumptionCalculator struct {
	repository                    repository.EnergyPrice
	providerName                  string
	batteries                     []*Battery
	enabled                       bool
	peakDetectionStdDevMultiplier float32
	socThreshold                  float32
}

// priceWithIndex is a helper struct for sorting prices by cost
type priceWithIndex struct {
	index int
	price float32
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
	}
}

// CalculateAddition determines how much power should be allocated to battery charging
// based on current electricity prices compared to available prices.
// It charges the battery only during the cheapest price periods available in the next 48 hours,
// and only if the entire charging duration can be completed within those cheapest periods.
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

	energyPrices, err := p.repository.EnergyPrices(now, endTime, p.providerName, prices.EnergyTypeElectricity)
	if err != nil {
		log.Warningf("Failed to retrieve energy prices for price-based charging: %v", err)
		return 0
	}

	if len(energyPrices) < 1 {
		log.Debugf("Insufficient price data for battery charging decision (got %d prices)", len(energyPrices))
		return 0
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

	// Calculate total charging duration needed for all batteries
	totalChargeDuration := time.Duration(0)
	for _, battery := range p.batteries {
		duration, _ := battery.ChargeDuration(availableChargePower, 100)
		if duration > 0 {
			totalChargeDuration += duration
		}
	}

	if totalChargeDuration == 0 {
		log.Debugf("No charging duration needed for batteries")
		return 0
	}

	// Find the cheapest slots that match the total charging duration
	cheapestSlots := findCheapestSlots(energyPrices, totalChargeDuration, p.batteries, p.peakDetectionStdDevMultiplier, p.socThreshold)
	if len(cheapestSlots) == 0 {
		log.Debugf("Could not find enough price periods for required charging duration of %v", totalChargeDuration)
		return 0
	}

	// Check if current price period is one of the cheapest slots
	isInCheapestSlots := false
	for _, slotIndex := range cheapestSlots {
		if slotIndex == currentPriceIndex {
			isInCheapestSlots = true
			break
		}
	}

	if !isInCheapestSlots {
		log.Debugf("Current time is not in the cheapest price slots, skipping battery charging")
		return 0
	}

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

		log.Debugf("Adding %d W for battery charging (duration: %v, current price: %.4f)",
			powerToAdd, chargeDuration, currentPrice.ConsumptionPrice)
	}

	return totalAddition
}

// findCheapestSlots returns the indices of price periods to charge during based on battery SoC.
// If SoC < socThreshold: Charges enough before expensive periods to survive them (survival strategy)
// If SoC >= socThreshold: Charges during the absolute cheapest periods (optimal strategy)
// peakDetectionStdDevMultiplier controls peak detection sensitivity (0.5=conservative, 1.0=optimal, 1.5=aggressive)
func findCheapestSlots(energyPrices []*prices.EnergyPrice, requiredDuration time.Duration, batteries []*Battery, stdDevMultiplier float32, socThreshold float32) []int {
	if len(energyPrices) == 0 || len(batteries) == 0 {
		return []int{}
	}

	// Calculate average SoC across all batteries
	totalSoC := float32(0)
	for _, battery := range batteries {
		totalSoC += battery.State().SoC()
	}
	avgSoC := totalSoC / float32(len(batteries))

	log.Debugf("Average battery SoC: %.2f%%", avgSoC)

	// Use different strategies based on SoC
	if avgSoC < socThreshold {
		log.Debugf("Low SoC (%.2f%%) detected, using survival charging strategy (threshold: %.2f%%)", avgSoC, socThreshold)
		return findSurvivalChargingSlots(energyPrices, requiredDuration, stdDevMultiplier)
	}

	log.Debugf("SoC sufficient (%.2f%%), using optimal charging strategy (threshold: %.2f%%)", avgSoC, socThreshold)
	return findOptimalChargingSlots(energyPrices, requiredDuration)
}

// findSurvivalChargingSlots prioritizes charging before expensive price periods.
// It identifies expensive periods using intelligent peak detection and charges
// just enough before them to survive the peak.
// peakDetectionStdDevMultiplier controls sensitivity: avg + (peakDetectionStdDevMultiplier × stdDev)
func findSurvivalChargingSlots(energyPrices []*prices.EnergyPrice, requiredDuration time.Duration, stdDevMultiplier float32) []int {
	if len(energyPrices) == 0 {
		return []int{}
	}

	// Calculate price statistics for intelligent peak detection
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

	// Calculate standard deviation for intelligent thresholding
	varianceSum := float32(0)
	for _, ep := range energyPrices {
		diff := ep.ConsumptionPrice - avgPrice
		varianceSum += diff * diff
	}
	stdDev := float32(math.Sqrt(float64(varianceSum / float32(len(energyPrices)))))

	log.Debugf("Price statistics: min=€%.4f, avg=€%.4f, max=€%.4f, stdDev=€%.4f",
		minPrice, avgPrice, maxPrice, stdDev)

	// Detect peaks: prices significantly above average using configurable multiplier
	// Formula: avg + (multiplier × stdDev)
	// multiplier 0.5: conservative (detects more peaks)
	// multiplier 1.0: optimal (balanced)
	// multiplier 1.5: aggressive (only extreme peaks)
	peakThreshold := avgPrice + (stdDevMultiplier * stdDev)

	log.Debugf("Peak detection threshold: €%.4f (avg + %.1f × stdDev)", peakThreshold, stdDevMultiplier)

	// Find consecutive peak periods (not just individual periods)
	peakPeriods := findPeakPeriods(energyPrices, peakThreshold)

	if len(peakPeriods) == 0 {
		log.Debugf("No significant peaks detected, falling back to optimal strategy")
		return findOptimalChargingSlots(energyPrices, requiredDuration)
	}

	log.Debugf("Found %d peak period blocks", len(peakPeriods))
	for i, period := range peakPeriods {
		peakPrice := energyPrices[period[0]].ConsumptionPrice
		for _, idx := range period {
			if energyPrices[idx].ConsumptionPrice > peakPrice {
				peakPrice = energyPrices[idx].ConsumptionPrice
			}
		}
		log.Debugf("Peak block %d: indices %d-%d, max price €%.4f",
			i+1, period[0], period[len(period)-1], peakPrice)
	}

	// Strategy: Charge before the first peak using cheapest available slots
	firstPeakStart := peakPeriods[0][0]
	selectedIndices := make([]int, 0)
	accumulatedDuration := time.Duration(0)

	// Get only prices before the first peak
	pricesWithIndices := make([]priceWithIndex, 0)
	for i := 0; i < firstPeakStart && i < len(energyPrices); i++ {
		pricesWithIndices = append(pricesWithIndices, priceWithIndex{
			index: i,
			price: energyPrices[i].ConsumptionPrice,
		})
	}

	// Sort by price
	sortByPrice(pricesWithIndices)

	// Select cheapest slots before the peak
	for _, pwi := range pricesWithIndices {
		selectedIndices = append(selectedIndices, pwi.index)
		accumulatedDuration += energyPrices[pwi.index].Duration()

		if accumulatedDuration >= requiredDuration {
			break
		}
	}

	// If we don't have enough time before the peak, also add cheap periods after
	if accumulatedDuration < requiredDuration {
		log.Debugf("Not enough cheap time before peak, adding periods after (accumulated: %v, need: %v)",
			accumulatedDuration, requiredDuration)

		lastPeakEnd := peakPeriods[len(peakPeriods)-1][len(peakPeriods[len(peakPeriods)-1])-1]

		pricesAfter := make([]priceWithIndex, 0)
		for i := lastPeakEnd + 1; i < len(energyPrices); i++ {
			pricesAfter = append(pricesAfter, priceWithIndex{
				index: i,
				price: energyPrices[i].ConsumptionPrice,
			})
		}
		sortByPrice(pricesAfter)

		for _, pwi := range pricesAfter {
			if accumulatedDuration >= requiredDuration {
				break
			}
			selectedIndices = append(selectedIndices, pwi.index)
			accumulatedDuration += energyPrices[pwi.index].Duration()
		}
	}

	log.Debugf("Survival charging: selected %d slots: need %v, have %v",
		len(selectedIndices), requiredDuration, accumulatedDuration)

	return selectedIndices
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

// findOptimalChargingSlots selects the absolute cheapest price periods.
func findOptimalChargingSlots(energyPrices []*prices.EnergyPrice, requiredDuration time.Duration) []int {
	if len(energyPrices) == 0 {
		return []int{}
	}

	pricesWithIndices := make([]priceWithIndex, len(energyPrices))
	for i := range energyPrices {
		pricesWithIndices[i] = priceWithIndex{
			index: i,
			price: energyPrices[i].ConsumptionPrice,
		}
	}

	// Sort by price (ascending)
	sortByPrice(pricesWithIndices)

	// Select the cheapest slots until we have enough duration
	accumulatedDuration := time.Duration(0)
	selectedIndices := make([]int, 0)

	for _, pwi := range pricesWithIndices {
		selectedIndices = append(selectedIndices, pwi.index)
		accumulatedDuration += energyPrices[pwi.index].Duration()

		if accumulatedDuration >= requiredDuration {
			break
		}
	}

	log.Debugf("Optimal charging: selected %d slots: need %v, have %v",
		len(selectedIndices), requiredDuration, accumulatedDuration)

	return selectedIndices
}

// sortByPrice sorts the slice of priceWithIndex in ascending order by price
func sortByPrice(prices []priceWithIndex) {
	for i := 0; i < len(prices); i++ {
		for j := i + 1; j < len(prices); j++ {
			if prices[j].price < prices[i].price {
				prices[i], prices[j] = prices[j], prices[i]
			}
		}
	}
}

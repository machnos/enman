package domain

import (
	"context"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// OptimalChargingPeriodCalculator calculates optimal charging periods based on energy prices
// It polls the price database hourly and identifies cheap price periods
type OptimalChargingPeriodCalculator struct {
	repository                    repository.EnergyPrice
	providerName                  string
	peakDetectionStdDevMultiplier float32
	roundTripEfficiency           float32 // Battery round-trip efficiency (0-100)
	ticker                        *time.Ticker
	tickerDoneChannel             chan bool
	currentPeriods                []*ChargingPeriod
	periodsLock                   sync.RWMutex
	lastCalculationTime           time.Time
	ctx                           context.Context
	scheduledEvents               sync.Map // Tracks scheduled period events to avoid duplicates
	survivalChargingSOCThreshold  float32  // Battery SOC threshold below which survival charging is enabled (0-100)
	system                        *System  // Reference to system for battery state access
}

// ChargingPeriod represents a continuous period with optimal prices for charging
type ChargingPeriod struct {
	StartTime time.Time
	EndTime   time.Time
}

func NewOptimalChargingPeriodCalculator(
	repository repository.EnergyPrice,
	providerName string,
	peakDetectionStdDevMultiplier float32,
	roundTripEfficiency float32,
	survivalChargingSOCThreshold float32,
	system *System,
) *OptimalChargingPeriodCalculator {
	if peakDetectionStdDevMultiplier <= 0 {
		peakDetectionStdDevMultiplier = 1.0
	}
	if roundTripEfficiency < 0 || roundTripEfficiency > 100 {
		roundTripEfficiency = 100 // Default to no losses
	}
	if survivalChargingSOCThreshold < 0 || survivalChargingSOCThreshold > 100 {
		survivalChargingSOCThreshold = 30 // Default to 30%
	}
	return &OptimalChargingPeriodCalculator{
		repository:                    repository,
		providerName:                  providerName,
		peakDetectionStdDevMultiplier: peakDetectionStdDevMultiplier,
		roundTripEfficiency:           roundTripEfficiency,
		survivalChargingSOCThreshold:  survivalChargingSOCThreshold,
		system:                        system,
		currentPeriods:                make([]*ChargingPeriod, 0),
	}
}

// SetSystem sets the system reference for accessing battery state during survival charging calculations
func (o *OptimalChargingPeriodCalculator) SetSystem(system *System) {
	o.system = system
}

// SetSurvivalChargingSOCThreshold sets the battery SoC threshold below which survival charging is enabled
func (o *OptimalChargingPeriodCalculator) SetSurvivalChargingSOCThreshold(threshold float32) {
	if threshold < 0 || threshold > 100 {
		threshold = 30
	}
	o.survivalChargingSOCThreshold = threshold
}

// Start begins the hourly polling of prices and calculation of optimal charging periods
// Returns when the context is cancelled (via errgroup coordination)
func (o *OptimalChargingPeriodCalculator) Start(ctx context.Context) error {
	if o.ticker != nil {
		// Already started
		return nil
	}
	log.Infof("Optimal charging period calculator started with price threshold multiplier: %.1f, and round-trip efficiency: %.1f%%",
		o.peakDetectionStdDevMultiplier, o.roundTripEfficiency)

	o.ctx = ctx
	o.ticker = time.NewTicker(5 * time.Minute)
	o.tickerDoneChannel = make(chan bool)

	// Initial calculation
	o.calculateOptimalPeriods()

	// Check if currently in an active charging period and fire start event if needed
	o.fireActiveChargingPeriodEventOnStart()

	// Run polling loop - this goroutine will block until context is cancelled
	for {
		select {
		case <-ctx.Done():
			o.cleanup()
			return nil
		case <-o.ticker.C:
			o.calculateOptimalPeriods()
		}
	}
}

// cleanup stops the hourly polling and cleans up resources
func (o *OptimalChargingPeriodCalculator) cleanup() {
	if o.ticker == nil {
		return
	}
	o.ticker.Stop()
	// Drain any pending signals
	select {
	case <-o.tickerDoneChannel:
	default:
	}
}

// fireActiveChargingPeriodEventOnStart checks if the application starts during an active charging period
// and fires the start event immediately if it does
func (o *OptimalChargingPeriodCalculator) fireActiveChargingPeriodEventOnStart() {
	now := time.Now()
	activePeriod := o.GetPeriodForTime(now)

	if activePeriod != nil {
		log.Infof("Application started during active charging period: %v to %v", activePeriod.StartTime, activePeriod.EndTime)

		// Fire the charging period start event immediately
		event := events.NewChargingPeriodValues().
			SetPeriodType(events.ChargingPeriodStart).
			SetStartTime(activePeriod.StartTime).
			SetEndTime(activePeriod.EndTime)

		log.Infof("Firing charging period START event on startup: %v to %v", activePeriod.StartTime, activePeriod.EndTime)
		events.ChargingPeriodChanges.Trigger(event)

		// Schedule the stop event for when the period ends
		o.scheduleChargingPeriodEvent(activePeriod, events.ChargingPeriodStop)
	}
}

// calculateOptimalPeriods fetches new prices and calculates optimal charging windows
func (o *OptimalChargingPeriodCalculator) calculateOptimalPeriods() {
	now := time.Now()

	// Fetch prices for today and tomorrow
	year, month, day := now.Date()
	startOfToday := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	endOfTomorrow := startOfToday.AddDate(0, 0, 2).Add(time.Nanosecond * -1)

	priceList, err := o.repository.EnergyPrices(startOfToday, endOfTomorrow, o.providerName, prices.EnergyTypeElectricity)
	if err != nil {
		log.Warningf("Failed to fetch energy prices for charging period calculation: %v", err)
		return
	}

	if len(priceList) == 0 {
		log.Debugf("No prices available for charging period calculation")
		return
	}

	// Only recalculate if we have new prices since last calculation
	if len(priceList) > 0 && priceList[len(priceList)-1].Time.Equal(o.lastCalculationTime) {
		log.Tracef("No new prices since last calculation at %v", o.lastCalculationTime)
		return
	}

	o.lastCalculationTime = now

	// Calculate the optimal charging periods
	newPeriods := o.identifyChargingPeriods(priceList, now)

	// Update current periods and fire events for changes
	o.updateChargingPeriods(newPeriods, now)
}

// identifyChargingPeriods analyzes prices and identifies optimal charging windows using a multi-strategy approach:
// 1. Identifies price peaks and finds charging windows BEFORE peaks to prepare for expensive periods
// 2. Identifies the lowest price periods throughout the day (if economically viable)
// 3. Identifies survival charging periods if battery SoC is below threshold
//
// This ensures we charge before peaks and also capture the absolute lowest prices for economic efficiency.
// Takes into account round-trip efficiency: a period is only economically viable if
// charging at the low price and discharging at the peak price yields net savings
// after accounting for battery losses.
func (o *OptimalChargingPeriodCalculator) identifyChargingPeriods(priceList []*prices.EnergyPrice, now time.Time) []*ChargingPeriod {
	if len(priceList) == 0 {
		return make([]*ChargingPeriod, 0)
	}

	// Sort prices by time to ensure proper ordering
	sort.Slice(priceList, func(i, j int) bool {
		return priceList[i].Time.Before(priceList[j].Time)
	})

	// Calculate mean, standard deviation, and max price
	meanPrice, stdDev, maxPrice := o.calculatePriceStatistics(priceList)

	// Calculate economically viable threshold accounting for round-trip efficiency
	efficiencyFactor := float64(o.roundTripEfficiency) / 100.0
	economicThreshold := maxPrice * efficiencyFactor

	// Identify peak threshold: prices above mean + stdDev
	peakThreshold := meanPrice + (stdDev * float64(o.peakDetectionStdDevMultiplier))

	// Identify cheap threshold: prices below mean - stdDev
	cheapThreshold := meanPrice - (stdDev * float64(o.peakDetectionStdDevMultiplier))

	log.Debugf("Price statistics - Mean: %.4f, StdDev: %.4f, Max: %.4f, Peak Threshold: %.4f, Cheap Threshold: %.4f, Economic Threshold (%.0f%% efficiency): %.4f",
		meanPrice, stdDev, maxPrice, peakThreshold, cheapThreshold, o.roundTripEfficiency, economicThreshold)

	// Identify peaks - consecutive expensive periods
	peaks := o.identifyPeaks(priceList, peakThreshold, now)
	log.Debugf("Identified %d price peaks", len(peaks))
	for _, peak := range peaks {
		log.Tracef("  Peak: %v to %v", peak.StartTime, peak.EndTime)
	}

	// Strategy 1: Find charging periods BEFORE peaks
	prePeakPeriods := o.findPrePeakChargingPeriods(priceList, peaks, economicThreshold, now)
	log.Debugf("Identified %d pre-peak charging periods", len(prePeakPeriods))

	// Strategy 2: Find the absolute lowest periods of the day (if economically viable)
	lowestPeriods := o.findLowestPriceChargingPeriods(priceList, cheapThreshold, economicThreshold, now)
	log.Debugf("Identified %d lowest-price charging periods", len(lowestPeriods))

	// Strategy 3: Find survival charging periods if battery is low
	survivalPeriods := o.findSurvivalChargingPeriods(priceList, peaks, now)
	log.Debugf("Identified %d survival charging periods", len(survivalPeriods))

	// Merge all periods (remove duplicates and overlaps)
	allPeriods := append(prePeakPeriods, lowestPeriods...)
	allPeriods = append(allPeriods, survivalPeriods...)
	mergedPeriods := o.mergePeriods(allPeriods)

	log.Debugf("Total identified charging periods after merging: %d", len(mergedPeriods))
	for _, period := range mergedPeriods {
		log.Tracef("  Period: %v to %v", period.StartTime, period.EndTime)
	}

	return mergedPeriods
}

// identifyPeaks finds consecutive periods where prices are above the peak threshold
// It also consolidates nearby peaks that are separated by short dips (< 45 minutes)
// to avoid fragmentation when a few low prices interrupt a generally high period
func (o *OptimalChargingPeriodCalculator) identifyPeaks(priceList []*prices.EnergyPrice, peakThreshold float64, now time.Time) []*ChargingPeriod {
	peaks := make([]*ChargingPeriod, 0)
	var consolidatedPeakStart *time.Time

	for i, price := range priceList {
		if price.Time.Before(now) {
			continue
		}

		isPeak := float64(price.ConsumptionPrice) > peakThreshold

		if isPeak {
			if consolidatedPeakStart == nil {
				t := price.Time
				consolidatedPeakStart = &t
			}
		} else {
			// Look ahead to see if there's another peak coming soon (within 45 minutes / 3 slots)
			hasNearbyPeak := false
			for j := i + 1; j < len(priceList) && j < i+3; j++ {
				if float64(priceList[j].ConsumptionPrice) > peakThreshold {
					hasNearbyPeak = true
					break
				}
			}

			if consolidatedPeakStart != nil {
				if hasNearbyPeak {
					// Continue the peak across this dip
					continue
				} else {
					// Close the consolidated peak
					peak := &ChargingPeriod{
						StartTime: *consolidatedPeakStart,
						EndTime:   price.Time,
					}
					peaks = append(peaks, peak)
					consolidatedPeakStart = nil
				}
			}
		}
	}

	if consolidatedPeakStart != nil {
		peak := &ChargingPeriod{
			StartTime: *consolidatedPeakStart,
			EndTime:   priceList[len(priceList)-1].EndTime,
		}
		peaks = append(peaks, peak)
	}

	return peaks
}

// findPrePeakChargingPeriods identifies charging periods immediately before each peak
// This allows the battery to be charged and ready before expensive periods
// The algorithm looks backwards from the peak start to find the longest continuous
// economical period that leads up to the peak
func (o *OptimalChargingPeriodCalculator) findPrePeakChargingPeriods(priceList []*prices.EnergyPrice, peaks []*ChargingPeriod, economicThreshold float64, now time.Time) []*ChargingPeriod {
	prePeakPeriods := make([]*ChargingPeriod, 0)

	for _, peak := range peaks {
		// Find consecutive economical prices leading up to this peak
		var periodStart *time.Time
		var periodEnd *time.Time

		// Work backwards from just before the peak
		for i := len(priceList) - 1; i >= 0; i-- {
			price := priceList[i]

			// Stop if we've gone before peak start
			if !price.Time.Before(peak.StartTime) {
				continue
			}

			// Stop if we've gone into the past
			if price.Time.Before(now) {
				break
			}

			// Check if this price is economically viable for charging
			isEconomical := float64(price.ConsumptionPrice) <= economicThreshold

			if isEconomical {
				// This is an economical price, extend the window
				if periodEnd == nil {
					t := price.Time
					periodEnd = &t
				}
				t := price.Time
				periodStart = &t
			} else {
				// Found a non-economical price
				if periodStart != nil && periodEnd != nil {
					// We have a complete economical window
					// Adjust end time to the price time (which is non-economical)
					period := &ChargingPeriod{
						StartTime: *periodStart,
						EndTime:   peak.StartTime,
					}

					// Only add if period is substantial (at least 15 minutes)
					if period.EndTime.Sub(period.StartTime) >= 15*time.Minute {
						prePeakPeriods = append(prePeakPeriods, period)
					}
					break
				}
				// Reset if we hit a non-economical price before finding any economical ones
				periodStart = nil
				periodEnd = nil
			}
		}

		// If we found economical prices all the way to the beginning
		if periodStart != nil && periodEnd != nil {
			period := &ChargingPeriod{
				StartTime: *periodStart,
				EndTime:   peak.StartTime,
			}
			if period.EndTime.Sub(period.StartTime) >= 15*time.Minute {
				prePeakPeriods = append(prePeakPeriods, period)
			}
		}
	}

	return prePeakPeriods
}

// findLowestPriceChargingPeriods identifies periods with the lowest prices (absolute cheap periods)
// These are periods where consecutive prices are significantly below the mean
func (o *OptimalChargingPeriodCalculator) findLowestPriceChargingPeriods(priceList []*prices.EnergyPrice, cheapThreshold float64, economicThreshold float64, now time.Time) []*ChargingPeriod {
	lowestPeriods := make([]*ChargingPeriod, 0)
	var periodStart *time.Time

	for _, price := range priceList {
		if price.Time.Before(now) {
			continue
		}

		// A period is a lowest-price period if it's both cheap AND economical
		isCheapAndEconomical := float64(price.ConsumptionPrice) <= cheapThreshold &&
			float64(price.ConsumptionPrice) <= economicThreshold

		if isCheapAndEconomical {
			if periodStart == nil {
				t := price.Time
				periodStart = &t
			}
		} else {
			if periodStart != nil {
				period := &ChargingPeriod{
					StartTime: *periodStart,
					EndTime:   price.Time,
				}
				lowestPeriods = append(lowestPeriods, period)
				periodStart = nil
			}
		}
	}

	if periodStart != nil {
		period := &ChargingPeriod{
			StartTime: *periodStart,
			EndTime:   priceList[len(priceList)-1].EndTime,
		}
		lowestPeriods = append(lowestPeriods, period)
	}

	return lowestPeriods
}

// findSurvivalChargingPeriods identifies charging periods before peaks when battery SoC is below threshold
// This ensures the battery is charged to survive peak pricing periods
func (o *OptimalChargingPeriodCalculator) findSurvivalChargingPeriods(priceList []*prices.EnergyPrice, peaks []*ChargingPeriod, now time.Time) []*ChargingPeriod {
	survivalPeriods := make([]*ChargingPeriod, 0)

	// Only enable survival charging if system and battery are available
	if o.system == nil || len(o.system.Batteries()) == 0 {
		return survivalPeriods
	}

	// Check current battery SoC
	battery := o.system.Batteries()[0]
	if !battery.IsMeasurementStarted() {
		return survivalPeriods
	}
	currentSoC := battery.State().SoC()

	// Only apply survival charging if battery is below threshold
	if currentSoC >= o.survivalChargingSOCThreshold {
		log.Tracef("Battery SoC (%.1f%%) is above survival threshold (%.1f%%), skipping survival charging", currentSoC, o.survivalChargingSOCThreshold)
		return survivalPeriods
	}

	log.Infof("Battery SoC (%.1f%%) is below survival threshold (%.1f%%), enabling survival charging", currentSoC, o.survivalChargingSOCThreshold)

	// Find charging windows before each peak when battery needs charging
	for _, peak := range peaks {
		// Find a reasonable window before the peak to charge (e.g., 2-4 hours before)
		windowStart := peak.StartTime.Add(-4 * time.Hour)
		windowEnd := peak.StartTime

		// Find the lowest priced continuous period within this window
		var lowestStart *time.Time
		var lowestEnd *time.Time
		lowestAvgPrice := float64(1.0)

		var currentStart *time.Time
		var currentSum float64
		var currentCount int

		for _, price := range priceList {
			if price.Time.Before(windowStart) || price.Time.After(windowEnd) {
				if currentStart != nil && currentCount > 0 {
					currentAvg := currentSum / float64(currentCount)
					if currentAvg < lowestAvgPrice {
						lowestAvgPrice = currentAvg
						lowestStart = currentStart
						t := price.Time
						lowestEnd = &t
					}
				}
				if price.Time.Before(windowStart) {
					currentStart = nil
					currentCount = 0
					currentSum = 0
				}
				continue
			}

			if currentStart == nil {
				t := price.Time
				currentStart = &t
			}
			currentSum += float64(price.ConsumptionPrice)
			currentCount++
		}

		if currentStart != nil && lowestStart != nil && lowestEnd != nil && lowestStart.Before(*lowestEnd) {
			period := &ChargingPeriod{
				StartTime: *lowestStart,
				EndTime:   *lowestEnd,
			}
			survivalPeriods = append(survivalPeriods, period)
		}
	}

	return survivalPeriods
}

// mergePeriods merges overlapping or adjacent periods
func (o *OptimalChargingPeriodCalculator) mergePeriods(periods []*ChargingPeriod) []*ChargingPeriod {
	if len(periods) == 0 {
		return make([]*ChargingPeriod, 0)
	}

	// Sort periods by start time
	sort.Slice(periods, func(i, j int) bool {
		return periods[i].StartTime.Before(periods[j].StartTime)
	})

	merged := make([]*ChargingPeriod, 0)
	current := periods[0]

	for i := 1; i < len(periods); i++ {
		next := periods[i]

		// If periods overlap or are adjacent (within 1 minute), merge them
		if next.StartTime.Sub(current.EndTime) <= time.Minute {
			if next.EndTime.After(current.EndTime) {
				current.EndTime = next.EndTime
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)
	return merged
}

// calculatePriceStatistics computes mean, standard deviation, and max price
func (o *OptimalChargingPeriodCalculator) calculatePriceStatistics(priceList []*prices.EnergyPrice) (float64, float64, float64) {
	if len(priceList) == 0 {
		return 0, 0, 0
	}

	// Calculate mean and find max
	var sum float64
	maxPrice := float64(priceList[0].ConsumptionPrice)
	for _, p := range priceList {
		price := float64(p.ConsumptionPrice)
		sum += price
		if price > maxPrice {
			maxPrice = price
		}
	}
	mean := sum / float64(len(priceList))

	// Calculate standard deviation
	var variance float64
	for _, p := range priceList {
		diff := float64(p.ConsumptionPrice) - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(priceList)))

	return mean, stdDev, maxPrice
}

// scheduleChargingPeriodEvent schedules a charging period event to fire at the specified time
// Similar to FirePriceChangedEvent in BasePriceImporter
func (o *OptimalChargingPeriodCalculator) scheduleChargingPeriodEvent(period *ChargingPeriod, eventType events.ChargingPeriodEventType) {
	if o.ctx == nil {
		log.Tracef("Cannot schedule event - context not initialized")
		return
	}

	// Determine which time to schedule the event for
	var eventTime time.Time
	var eventKey string

	if eventType == events.ChargingPeriodStart {
		eventTime = period.StartTime
		eventKey = fmt.Sprintf("charging-start-%v-%v", period.StartTime, period.EndTime)
	} else {
		eventTime = period.EndTime
		eventKey = fmt.Sprintf("charging-stop-%v-%v", period.StartTime, period.EndTime)
	}

	// Check if event is in the past
	if time.Now().After(eventTime) {
		log.Tracef("Not scheduling charging period event because it is in the past: %s", eventKey)
		return
	}

	// Check if event is already scheduled
	_, ok := o.scheduledEvents.Load(eventKey)
	if ok {
		log.Tracef("Not scheduling charging period event because it was already scheduled: %s", eventKey)
		return
	}

	log.Tracef("Scheduling charging period event: %s", eventKey)
	o.scheduledEvents.Store(eventKey, true)

	// Schedule the event to fire at the specified time
	go func() {
		timer := time.NewTimer(time.Until(eventTime))
		defer timer.Stop()

		select {
		case <-timer.C:
			event := events.NewChargingPeriodValues().
				SetPeriodType(eventType).
				SetStartTime(period.StartTime).
				SetEndTime(period.EndTime)

			if eventType == events.ChargingPeriodStart {
				log.Infof("Firing charging period START event: %v to %v", period.StartTime, period.EndTime)
			} else {
				log.Infof("Firing charging period STOP event: %v to %v", period.StartTime, period.EndTime)
			}

			events.ChargingPeriodChanges.Trigger(event)
			o.scheduledEvents.Delete(eventKey)
			return

		case <-o.ctx.Done():
			o.scheduledEvents.Delete(eventKey)
			return
		}
	}()
}

// updateChargingPeriods compares new periods with current ones and schedules events
func (o *OptimalChargingPeriodCalculator) updateChargingPeriods(newPeriods []*ChargingPeriod, now time.Time) {
	o.periodsLock.Lock()
	defer o.periodsLock.Unlock()

	// Find periods that have started - schedule start events
	for _, newPeriod := range newPeriods {
		found := false
		for _, currentPeriod := range o.currentPeriods {
			if newPeriod.StartTime.Equal(currentPeriod.StartTime) {
				found = true
				break
			}
		}

		// New period that wasn't in current list - schedule start event
		if !found && newPeriod.StartTime.After(now) {
			log.Infof("Optimal charging period identified: %v to %v", newPeriod.StartTime, newPeriod.EndTime)
			o.scheduleChargingPeriodEvent(newPeriod, events.ChargingPeriodStart)
			o.scheduleChargingPeriodEvent(newPeriod, events.ChargingPeriodStop)
		}
	}

	// Find periods that have ended - cancel scheduled stop events if period is removed
	for _, currentPeriod := range o.currentPeriods {
		found := false
		for _, newPeriod := range newPeriods {
			if currentPeriod.StartTime.Equal(newPeriod.StartTime) {
				found = true
				break
			}
		}

		// Period no longer in new list
		if !found && currentPeriod.EndTime.After(now) {
			log.Infof("Optimal charging period removed: %v to %v", currentPeriod.StartTime, currentPeriod.EndTime)
			// Remove any scheduled events for this period
			eventKey := fmt.Sprintf("charging-stop-%v-%v", currentPeriod.StartTime, currentPeriod.EndTime)
			o.scheduledEvents.Delete(eventKey)
		}
	}

	o.currentPeriods = newPeriods
}

// GetCurrentPeriods returns the currently active charging periods
func (o *OptimalChargingPeriodCalculator) GetCurrentPeriods() []*ChargingPeriod {
	o.periodsLock.RLock()
	defer o.periodsLock.RUnlock()

	periods := make([]*ChargingPeriod, len(o.currentPeriods))
	copy(periods, o.currentPeriods)
	return periods
}

// IsInChargingPeriod checks if the given time is within an optimal charging period
func (o *OptimalChargingPeriodCalculator) IsInChargingPeriod(t time.Time) bool {
	o.periodsLock.RLock()
	defer o.periodsLock.RUnlock()

	for _, period := range o.currentPeriods {
		if t.After(period.StartTime) && t.Before(period.EndTime) {
			return true
		}
	}
	return false
}

// GetPeriodForTime returns the charging period that contains the given time, or nil
func (o *OptimalChargingPeriodCalculator) GetPeriodForTime(t time.Time) *ChargingPeriod {
	o.periodsLock.RLock()
	defer o.periodsLock.RUnlock()

	for _, period := range o.currentPeriods {
		if !t.Before(period.StartTime) && t.Before(period.EndTime) {
			return period
		}
	}
	return nil
}

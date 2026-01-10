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
) *OptimalChargingPeriodCalculator {
	if peakDetectionStdDevMultiplier <= 0 {
		peakDetectionStdDevMultiplier = 1.0
	}
	if roundTripEfficiency < 0 || roundTripEfficiency > 100 {
		roundTripEfficiency = 100 // Default to no losses
	}
	return &OptimalChargingPeriodCalculator{
		repository:                    repository,
		providerName:                  providerName,
		peakDetectionStdDevMultiplier: peakDetectionStdDevMultiplier,
		roundTripEfficiency:           roundTripEfficiency,
		currentPeriods:                make([]*ChargingPeriod, 0),
	}
}

// Start begins the hourly polling of prices and calculation of optimal charging periods
func (o *OptimalChargingPeriodCalculator) Start(ctx context.Context) {
	if o.ticker != nil {
		// Already started
		return
	}

	o.ctx = ctx
	o.ticker = time.NewTicker(1 * time.Hour)
	o.tickerDoneChannel = make(chan bool)

	// Initial calculation
	o.calculateOptimalPeriods()

	// Check if currently in an active charging period and fire start event if needed
	o.fireActiveChargingPeriodEventOnStart()

	go func() {
		for {
			select {
			case <-ctx.Done():
				o.Stop()
				return
			case <-o.ticker.C:
				o.calculateOptimalPeriods()
			}
		}
	}()
}

// Stop stops the hourly polling
func (o *OptimalChargingPeriodCalculator) Stop() {
	if o.ticker == nil {
		return
	}
	o.ticker.Stop()
	o.tickerDoneChannel <- true
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

// identifyChargingPeriods analyzes prices and identifies optimal charging windows
// Takes into account round-trip efficiency: a period is only economically viable if
// charging at the low price and discharging at the peak price yields net savings
// after accounting for battery losses
func (o *OptimalChargingPeriodCalculator) identifyChargingPeriods(priceList []*prices.EnergyPrice, now time.Time) []*ChargingPeriod {
	if len(priceList) == 0 {
		return make([]*ChargingPeriod, 0)
	}

	// Sort prices by time to ensure proper ordering
	sort.Slice(priceList, func(i, j int) bool {
		return priceList[i].Time.Before(priceList[j].Time)
	})

	// Calculate mean and standard deviation of prices
	meanPrice, stdDev := o.calculatePriceStatistics(priceList)

	// Identify price threshold for cheap periods (mean - stdDev * multiplier)
	priceThreshold := meanPrice - (stdDev * float64(o.peakDetectionStdDevMultiplier))

	// Calculate economically viable threshold accounting for round-trip efficiency
	// When charging at priceThreshold and discharging at meanPrice, the effective cost
	// is: (cost_to_charge / round_trip_efficiency). This must be less than peak price
	// for it to be economically viable.
	// Formula: cost_to_charge < (peak_price * round_trip_efficiency / 100)
	efficiencyFactor := float64(o.roundTripEfficiency) / 100.0
	economicThreshold := meanPrice * efficiencyFactor

	log.Debugf("Price statistics - Mean: %.4f, StdDev: %.4f, Base Threshold: %.4f, Economic Threshold (%.0f%% efficiency): %.4f",
		meanPrice, stdDev, priceThreshold, o.roundTripEfficiency, economicThreshold)

	// Group consecutive cheap periods
	periods := make([]*ChargingPeriod, 0)
	var periodStart *time.Time

	for _, price := range priceList {
		// Skip prices in the past
		if price.Time.Before(now) {
			continue
		}

		// A period is economically viable if the price is low enough AND
		// it's below the economic threshold accounting for battery losses
		isCheap := float64(price.ConsumptionPrice) <= priceThreshold &&
			float64(price.ConsumptionPrice) <= economicThreshold

		if isCheap {
			if periodStart == nil {
				// Start new period
				t := price.Time
				periodStart = &t
			}
		} else {
			// End current period if exists
			if periodStart != nil {
				period := &ChargingPeriod{
					StartTime: *periodStart,
					EndTime:   price.Time,
				}
				periods = append(periods, period)

				periodStart = nil
			}
		}
	}

	// Handle case where last prices are cheap (period extends to end of data)
	if periodStart != nil {
		period := &ChargingPeriod{
			StartTime: *periodStart,
			EndTime:   priceList[len(priceList)-1].Time.Add(time.Hour),
		}
		periods = append(periods, period)
	}

	return periods
}

// calculatePriceStatistics computes mean and standard deviation of prices
func (o *OptimalChargingPeriodCalculator) calculatePriceStatistics(priceList []*prices.EnergyPrice) (float64, float64) {
	if len(priceList) == 0 {
		return 0, 0
	}

	// Calculate mean
	var sum float64
	for _, p := range priceList {
		sum += float64(p.ConsumptionPrice)
	}
	mean := sum / float64(len(priceList))

	// Calculate standard deviation
	var variance float64
	for _, p := range priceList {
		diff := float64(p.ConsumptionPrice) - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(priceList)))

	return mean, stdDev
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
			log.Infof("Optimal charging period ended: %v to %v", currentPeriod.StartTime, currentPeriod.EndTime)
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

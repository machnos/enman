package domain

import (
	"context"
	"enman/internal/domain/battery"
	"enman/internal/domain/events"
	"enman/internal/log"
	"sync"
	"time"
)

// PriceBasedElectricityConsumptionCalculator listens to battery schedule slot events
// and provides the scheduled charge/discharge power to the GridTargetConsumptionCalculator.
// It bridges the BatteryScheduleOptimizer with the grid controller.
type PriceBasedElectricityConsumptionCalculator struct {
	mu            sync.RWMutex
	scheduleSlots map[time.Time]*battery.ScheduleSlot // Keyed by start time
	cancel        context.CancelFunc
}

func NewPriceBasedElectricityConsumptionCalculator() *PriceBasedElectricityConsumptionCalculator {
	return &PriceBasedElectricityConsumptionCalculator{
		scheduleSlots: make(map[time.Time]*battery.ScheduleSlot),
	}
}

// Start registers the event listener for battery schedule slot events
func (p *PriceBasedElectricityConsumptionCalculator) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)

	// Register to receive battery schedule slot events
	events.BatteryScheduleSlots.Register(p, func(event *events.BatteryScheduleSlotEvent) bool {
		// Accept all battery schedule slot events
		return event.Valid()
	})

	log.Info("PriceBasedElectricityConsumptionCalculator started and listening for battery schedule events")

	// Start a goroutine to clean up old schedule slots periodically
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.cleanupOldSlots()
			}
		}
	}()
}

// Stop unregisters the event listener
func (p *PriceBasedElectricityConsumptionCalculator) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	events.BatteryScheduleSlots.Deregister(p)
	log.Info("PriceBasedElectricityConsumptionCalculator stopped")
}

// HandleEvent implements the BatteryScheduleSlotListener interface
func (p *PriceBasedElectricityConsumptionCalculator) HandleEvent(event *events.BatteryScheduleSlotEvent) {
	if event == nil || !event.Valid() {
		return
	}

	slot := event.Slot()
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store the slot, replacing any existing slot with the same start time
	p.scheduleSlots[slot.StartTime()] = slot

	if log.DebugEnabled() {
		action := "idle"
		if slot.ChargePower() > 0 {
			action = "charging"
		} else if slot.ChargePower() < 0 {
			action = "discharging"
		}
		log.Debugf("PriceBasedElectricityConsumptionCalculator: received schedule slot %v-%v, power=%.0fW (%s), source=%s",
			slot.StartTime().Format("15:04"), slot.EndTime().Format("15:04"),
			slot.ChargePower(), action, slot.ChargingSource())
	}
}

// CalculateAddition returns the additional power to draw from/feed to grid
// based on the current battery schedule.
// Positive = draw from grid (charge battery), Negative = feed to grid (discharge battery)
func (p *PriceBasedElectricityConsumptionCalculator) CalculateAddition(availableChargePower float32) float32 {
	now := time.Now()

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Find the currently active schedule slot
	for _, slot := range p.scheduleSlots {
		if now.After(slot.StartTime()) && now.Before(slot.EndTime()) || now.Equal(slot.StartTime()) {
			chargePower := slot.ChargePower()

			// If charging, limit to available power
			if chargePower > 0 && chargePower > availableChargePower {
				chargePower = availableChargePower
			}

			if log.DebugEnabled() {
				log.Debugf("PriceBasedElectricityConsumptionCalculator: active slot found, returning power=%.0fW", chargePower)
			}

			return chargePower
		}
	}

	// No active schedule slot found
	return 0
}

// cleanupOldSlots removes schedule slots that have already ended
func (p *PriceBasedElectricityConsumptionCalculator) cleanupOldSlots() {
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	for startTime, slot := range p.scheduleSlots {
		if slot.EndTime().Before(now) {
			delete(p.scheduleSlots, startTime)
		}
	}
}

// GetCurrentSlot returns the currently active schedule slot, if any (for testing/debugging)
func (p *PriceBasedElectricityConsumptionCalculator) GetCurrentSlot() *battery.ScheduleSlot {
	now := time.Now()

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, slot := range p.scheduleSlots {
		if now.After(slot.StartTime()) && now.Before(slot.EndTime()) || now.Equal(slot.StartTime()) {
			return slot
		}
	}

	return nil
}

// GetScheduledSlots returns all stored schedule slots (for testing/debugging)
func (p *PriceBasedElectricityConsumptionCalculator) GetScheduledSlots() []*battery.ScheduleSlot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	slots := make([]*battery.ScheduleSlot, 0, len(p.scheduleSlots))
	for _, slot := range p.scheduleSlots {
		slots = append(slots, slot)
	}
	return slots
}

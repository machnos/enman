package domain

import (
	"context"
	"enman/internal/domain/buckets"
	"enman/internal/domain/events"
	"enman/internal/log"
	"sync/atomic"
	"time"
)

// BatterySchedulerConfig controls the BatteryScheduler service lifecycle and
// safety behaviour.
type BatterySchedulerConfig struct {
	BucketSize               time.Duration
	BatteryRestartPercentage float32 // mirrors PvStateController for safety; below this, discharges are blocked
	BatteryCutoffPercentage  float32 // mirrors PvStateController for safety; above this, charges are blocked when grid lost
}

// BatteryScheduler orchestrates the per-bucket battery bias evaluation. It does
// not plan itself; it consumes the battery forecasts produced by
// ForecasterService (persisted via the Forecasts event) and exposes the active
// bucket's signed power as the bias for the setpoint aggregator. Safety
// filtering (grid-loss, SoC bounds) is applied here.
type BatteryScheduler struct {
	system *System
	repo   Repository
	cfg    BatterySchedulerConfig

	// activeBiasW is the currently active battery bias in watts.
	// Positive = pull from grid into the battery (force charge).
	// Negative = push from battery to grid (force discharge).
	activeBiasW atomic.Value // float32
}

func NewBatteryScheduler(system *System, repo Repository, cfg BatterySchedulerConfig) *BatteryScheduler {
	if cfg.BucketSize <= 0 {
		cfg.BucketSize = buckets.DefaultBucketSize
	}
	s := &BatteryScheduler{
		system: system,
		repo:   repo,
		cfg:    cfg,
	}
	s.activeBiasW.Store(float32(0))
	return s
}

// ActiveBiasW returns the currently active battery bias in watts.
// Positive = forced charge (grid → battery), negative = forced discharge (battery → grid).
// Safety filtering is applied here so callers always get a value safe to apply.
func (s *BatteryScheduler) ActiveBiasW() float32 {
	bias := s.activeBiasW.Load().(float32)
	if bias == 0 {
		return 0
	}
	if s.system.Grid() != nil && bias > 0 {
		if controller := s.system.Grid().controller; controller != nil {
			connected, err := controller.GridConnected()
			if err == nil && !connected {
				return 0
			}
		}
	}
	if bias < 0 && s.cfg.BatteryRestartPercentage > 0 {
		for _, b := range s.system.Batteries() {
			if b.State() != nil && b.State().SoC() < s.cfg.BatteryRestartPercentage {
				return 0
			}
		}
	}
	if bias > 0 && s.cfg.BatteryCutoffPercentage > 0 {
		allFull := true
		for _, b := range s.system.Batteries() {
			if b.State() == nil || b.State().SoC() < s.cfg.BatteryCutoffPercentage {
				allFull = false
				break
			}
		}
		if allFull {
			return 0
		}
	}
	return bias
}

// Start subscribes to active forecast events to update the bias and runs the
// per-bucket boundary loop as a fallback. It returns when ctx is cancelled.
func (s *BatteryScheduler) Start(ctx context.Context) error {
	s.refreshActiveBias()

	listener := &batterySchedulerForecastListener{s: s}
	events.Forecasts.Register(listener, func(values *events.ForecastValues) bool {
		return values != nil && values.Kind() == events.ForecastKindBattery
	})

	for {
		next := buckets.Align(time.Now(), s.cfg.BucketSize).Add(s.cfg.BucketSize)
		wait := time.Until(next)
		if wait <= 0 {
			wait = time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			events.Forecasts.Deregister(listener)
			return nil
		case <-timer.C:
			s.refreshActiveBias()
		}
	}
}

// refreshActiveBias reads the active battery forecast for now and updates
// activeBiasW to the sum of per-battery Watts values.
func (s *BatteryScheduler) refreshActiveBias() {
	now := time.Now()
	bias := float32(0)
	hadActive := false
	for _, b := range s.system.Batteries() {
		rec, err := s.repo.ForecastAtTime(now, "", string(events.ForecastKindBattery), b.Name(), LessOrEqual)
		if err != nil {
			log.Debugf("scheduler: failed to load battery forecast for %s: %v", b.Name(), err)
			continue
		}
		if rec == nil {
			continue
		}
		// Only count if the bucket actually contains 'now'.
		if now.Before(rec.BucketStart) || !now.Before(rec.BucketStart.Add(rec.BucketSize)) {
			continue
		}
		bias += rec.Watts
		hadActive = true
	}
	if !hadActive {
		s.activeBiasW.Store(float32(0))
		return
	}
	s.activeBiasW.Store(bias)
}

// batterySchedulerForecastListener triggers a bias refresh on each new battery
// forecast event (filtered to ForecastKindBattery via the registration filter).
type batterySchedulerForecastListener struct{ s *BatteryScheduler }

func (l *batterySchedulerForecastListener) HandleEvent(values *events.ForecastValues) {
	if values == nil {
		return
	}
	l.s.refreshActiveBias()
}

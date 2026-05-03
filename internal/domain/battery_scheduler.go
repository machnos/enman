package domain

import (
	"context"
	"enman/internal/domain/buckets"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// BatterySchedulerConfig controls the BatteryScheduler service lifecycle and
// safety behaviour.
type BatterySchedulerConfig struct {
	OptimizerConfig
	ProviderName             string        // electricity provider whose price curve we use
	ReoptimizationInterval   time.Duration // minimum time between two re-optimizations
	BatteryRestartPercentage float32       // mirrors PvStateController for safety; below this, discharges are blocked
	BatteryCutoffPercentage  float32       // mirrors PvStateController for safety; above this, charges are blocked when grid lost
}

// BatteryScheduler is the orchestrator for price-aware battery scheduling.
// It listens to EnergyPrices and Forecasts events, debounces re-optimizations,
// persists schedules, and exposes the current battery-charging bias (in watts)
// for the setpoint aggregator to consume.
type BatteryScheduler struct {
	system    *System
	repo      Repository
	optimizer Optimizer
	cfg       BatterySchedulerConfig

	// activeBiasW is the currently active battery bias in watts.
	// Positive = pull from grid into the battery (force charge).
	// Negative = push from battery to grid (force discharge).
	activeBiasW atomic.Value // float32

	mu             sync.Mutex
	pendingRequest atomic.Bool
	lastRun        time.Time
}

func NewBatteryScheduler(system *System, repo Repository, optimizer Optimizer, cfg BatterySchedulerConfig) *BatteryScheduler {
	if cfg.ReoptimizationInterval <= 0 {
		cfg.ReoptimizationInterval = 5 * time.Minute
	}
	if cfg.BucketSize <= 0 {
		cfg.BucketSize = buckets.DefaultBucketSize
	}
	if cfg.Horizon <= 0 {
		cfg.Horizon = 36 * time.Hour
	}
	s := &BatteryScheduler{
		system:    system,
		repo:      repo,
		optimizer: optimizer,
		cfg:       cfg,
	}
	s.activeBiasW.Store(float32(0))
	return s
}

// ActiveBiasW returns the currently active battery bias in watts.
// Positive = forced charge (grid → battery), negative = forced discharge (battery → grid).
// Safety filtering (grid loss, SoC bounds) is applied here so callers always
// get a value safe to apply.
func (s *BatteryScheduler) ActiveBiasW() float32 {
	bias := s.activeBiasW.Load().(float32)
	if bias == 0 {
		return 0
	}

	// Safety: if grid is lost and we'd try to charge from grid, drop bias.
	if s.system.Grid() != nil && bias > 0 {
		if controller := s.system.Grid().controller; controller != nil {
			connected, err := controller.GridConnected()
			if err == nil && !connected {
				return 0
			}
		}
	}
	// Safety: if discharging and any battery is at/below restart SoC, drop bias.
	if bias < 0 && s.cfg.BatteryRestartPercentage > 0 {
		for _, b := range s.system.Batteries() {
			if b.State() != nil && b.State().SoC() < s.cfg.BatteryRestartPercentage {
				return 0
			}
		}
	}
	// Safety: if charging and all batteries are at/above cutoff SoC, drop bias.
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

// Start subscribes to triggering events and runs the active-bucket loop.
// It returns when ctx is cancelled.
func (s *BatteryScheduler) Start(ctx context.Context) error {
	if s.optimizer == nil {
		log.Warning("BatteryScheduler started without an optimizer; nothing to do.")
		<-ctx.Done()
		return nil
	}
	// Restart recovery: pick up any persisted schedule that is currently active.
	s.recoverActiveBias()
	// Subscribe to triggers.
	priceListener := &batterySchedulerPriceListener{s: s}
	forecastListener := &batterySchedulerForecastListener{s: s}
	events.EnergyPrices.Register(priceListener, func(values *events.EnergyPriceValues) bool {
		return values.EnergyType() == prices.EnergyTypeElectricity
	})
	events.Forecasts.Register(forecastListener, nil)

	// Run an initial optimization shortly after startup once the system has
	// a chance to read state.
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			s.requestRun(ctx)
		}
	}()

	// Active-bucket loop: every bucket boundary, re-evaluate the active bias.
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
			events.EnergyPrices.Deregister(priceListener)
			events.Forecasts.Deregister(forecastListener)
			return nil
		case <-timer.C:
			s.refreshActiveBias()
		}
	}
}

// requestRun debounces re-optimization to once per ReoptimizationInterval.
func (s *BatteryScheduler) requestRun(ctx context.Context) {
	if s.pendingRequest.Swap(true) {
		// already pending
		return
	}
	go func() {
		defer s.pendingRequest.Store(false)
		s.mu.Lock()
		wait := time.Duration(0)
		if !s.lastRun.IsZero() {
			elapsed := time.Since(s.lastRun)
			if elapsed < s.cfg.ReoptimizationInterval {
				wait = s.cfg.ReoptimizationInterval - elapsed
			}
		}
		s.mu.Unlock()
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
		s.run(ctx)
	}()
}

// run fetches inputs, plans, persists and broadcasts. It also caches the
// active record per battery for the per-bucket loop to read.
func (s *BatteryScheduler) run(ctx context.Context) {
	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()

	now := time.Now()
	from := buckets.Align(now, s.cfg.BucketSize)
	till := from.Add(s.cfg.Horizon)
	priceCurve, err := s.repo.EnergyPrices(from, till, s.cfg.ProviderName, prices.EnergyTypeElectricity)
	if err != nil {
		log.Warningf("scheduler: failed to load energy prices: %v", err)
		return
	}
	loadFcst, err := s.repo.Forecasts(from, till, "", string(events.ForecastKindLoad))
	if err != nil {
		log.Debugf("scheduler: failed to load load forecasts: %v", err)
	}
	pvFcst, err := s.repo.Forecasts(from, till, "", string(events.ForecastKindPv))
	if err != nil {
		log.Debugf("scheduler: failed to load pv forecasts: %v", err)
	}

	// Pick the most recent generation per (kind, bucket).
	loadFcst = pickLatestForecasts(loadFcst)
	pvFcst = pickLatestForecasts(pvFcst)

	records := s.optimizer.Plan(ctx, priceCurve, loadFcst, pvFcst, s.system.Batteries(), s.cfg.OptimizerConfig)
	if len(records) == 0 {
		return
	}
	for _, r := range records {
		if r == nil {
			continue
		}
		if err := s.repo.StoreBatterySchedule(r); err != nil {
			log.Debugf("scheduler: failed to store schedule (%s @ %s): %v", r.BatteryName, r.BucketStart, err)
		}
		events.BatterySchedules.Trigger(events.NewBatteryScheduleValues().
			SetGeneratedAt(r.GeneratedAt).
			SetBatteryName(r.BatteryName).
			SetBucketStart(r.BucketStart).
			SetBucketSize(r.BucketSize).
			SetAction(events.BatteryAction(r.Action)).
			SetPowerW(r.PowerW).
			SetPredictedSoC(r.PredictedSoC).
			SetReason(r.Reason).
			SetActive(false))
	}
	// Refresh the active bias in case the new plan changed the current bucket.
	s.refreshActiveBias()
}

// refreshActiveBias reads the active schedule record(s) for now and updates
// activeBiasW to the sum of per-battery PowerW values.
func (s *BatteryScheduler) refreshActiveBias() {
	now := time.Now()
	bias := float32(0)
	hadActive := false
	for _, b := range s.system.Batteries() {
		rec, err := s.repo.BatteryScheduleAtTime(now, b.Name(), LessOrEqual)
		if err != nil || rec == nil {
			continue
		}
		// Only count if the bucket actually contains 'now'.
		if now.Before(rec.BucketStart) || !now.Before(rec.BucketStart.Add(rec.BucketSize)) {
			continue
		}
		bias += rec.PowerW
		hadActive = true
		// Fan out the active replay event for downstream listeners (UI etc).
		events.BatterySchedules.Trigger(events.NewBatteryScheduleValues().
			SetGeneratedAt(rec.GeneratedAt).
			SetBatteryName(rec.BatteryName).
			SetBucketStart(rec.BucketStart).
			SetBucketSize(rec.BucketSize).
			SetAction(events.BatteryAction(rec.Action)).
			SetPowerW(rec.PowerW).
			SetPredictedSoC(rec.PredictedSoC).
			SetReason(rec.Reason).
			SetActive(true))
	}
	if !hadActive {
		s.activeBiasW.Store(float32(0))
		return
	}
	s.activeBiasW.Store(bias)
}

func (s *BatteryScheduler) recoverActiveBias() {
	s.refreshActiveBias()
}

// pickLatestForecasts groups by bucket start (per kind) and keeps the record
// with the most recent GeneratedAt. Useful when several forecast generations
// coexist in the DB.
func pickLatestForecasts(records []*ForecastRecord) []*ForecastRecord {
	bestByBucket := make(map[time.Time]*ForecastRecord, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		if existing, ok := bestByBucket[r.BucketStart]; !ok || r.GeneratedAt.After(existing.GeneratedAt) {
			bestByBucket[r.BucketStart] = r
		}
	}
	out := make([]*ForecastRecord, 0, len(bestByBucket))
	for _, r := range bestByBucket {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BucketStart.Before(out[j].BucketStart) })
	return out
}

// batterySchedulerPriceListener and batterySchedulerForecastListener are
// thin event-handler adapters. They are pointer types so the events package's
// listener-keyed map handles registration correctly.
type batterySchedulerPriceListener struct{ s *BatteryScheduler }

func (l *batterySchedulerPriceListener) HandleEvent(_ *events.EnergyPriceValues) {
	l.s.requestRun(context.Background())
}

type batterySchedulerForecastListener struct{ s *BatteryScheduler }

func (l *batterySchedulerForecastListener) HandleEvent(values *events.ForecastValues) {
	if values == nil || values.Active() {
		// We only re-plan on new generations, not on per-bucket replays.
		return
	}
	l.s.requestRun(context.Background())
}

package domain

import (
	"context"
	"enman/internal/domain/buckets"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/log"
	"sync"
	"time"
)

// Forecaster produces forecast buckets for [from, till) at a given bucket size.
// Implementations must be deterministic and side-effect free; persistence and
// event broadcasting is handled by ForecasterService.
type Forecaster interface {
	Name() string
	Forecast(ctx context.Context, from, till time.Time, bucketSize time.Duration) ([]*ForecastRecord, error)
}

// ForecasterService runs all configured forecasters periodically.
type ForecasterService struct {
	system      *System
	repo        Repository
	forecasters []Forecaster
	bucketSize  time.Duration
	horizon     time.Duration
	interval    time.Duration

	mu             sync.Mutex
	activeRecords  map[string]*ForecastRecord // key = kind|bucket-rfc3339
	nextActiveTime time.Time
}

// NewForecasterService creates a new service. bucketSize/horizon/interval must be > 0.
func NewForecasterService(system *System, repo Repository, bucketSize, horizon, interval time.Duration, forecasters ...Forecaster) *ForecasterService {
	if bucketSize <= 0 {
		bucketSize = buckets.DefaultBucketSize
	}
	if horizon <= 0 {
		horizon = 36 * time.Hour
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &ForecasterService{
		system:        system,
		repo:          repo,
		forecasters:   forecasters,
		bucketSize:    bucketSize,
		horizon:       horizon,
		interval:      interval,
		activeRecords: make(map[string]*ForecastRecord),
	}
}

// Start runs the forecaster loop and a per-bucket "active" event fan-out
// loop. It returns when ctx is cancelled. Suitable for use with errgroup.
func (s *ForecasterService) Start(ctx context.Context) error {
	if len(s.forecasters) == 0 {
		log.Warning("ForecasterService started without any forecasters; nothing will be produced.")
		<-ctx.Done()
		return nil
	}
	// Run once immediately, then on the configured interval.
	s.runOnce(ctx)
	go s.activeLoop(ctx)
	tick := time.NewTicker(s.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce generates forecasts for the configured horizon, persists them and
// broadcasts a "new" event per bucket.
func (s *ForecasterService) runOnce(ctx context.Context) {
	now := time.Now()
	from := buckets.Align(now, s.bucketSize)
	till := from.Add(s.horizon)
	for _, f := range s.forecasters {
		records, err := f.Forecast(ctx, from, till, s.bucketSize)
		if err != nil {
			log.Warningf("Forecaster %s failed: %v", f.Name(), err)
			continue
		}
		for _, r := range records {
			if r == nil {
				continue
			}
			if err := s.repo.StoreForecast(r); err != nil {
				log.Debugf("Failed to store forecast (%s/%s @ %s): %v", r.ModelName, r.Kind, r.BucketStart, err)
			}
			events.Forecasts.Trigger(events.NewForecastValues().
				SetGeneratedAt(r.GeneratedAt).
				SetModelName(r.ModelName).
				SetKind(events.ForecastKind(r.Kind)).
				SetBucketStart(r.BucketStart).
				SetBucketSize(r.BucketSize).
				SetWatts(r.Watts).
				SetConfidence(r.Confidence).
				SetActive(false))
		}
		s.cacheActiveRecords(records)
	}
}

func (s *ForecasterService) cacheActiveRecords(records []*ForecastRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, r := range records {
		if r == nil || r.BucketStart.Before(now) {
			continue
		}
		key := r.Kind + "|" + r.BucketStart.UTC().Format(time.RFC3339Nano)
		// Only overwrite if newer generation.
		if existing, ok := s.activeRecords[key]; !ok || r.GeneratedAt.After(existing.GeneratedAt) {
			s.activeRecords[key] = r
		}
	}
}

// activeLoop wakes at every bucket boundary and fires "active" events for
// the records whose BucketStart matches the current bucket. Cheap single
// goroutine instead of one timer per future bucket.
func (s *ForecasterService) activeLoop(ctx context.Context) {
	for {
		next := buckets.Align(time.Now(), s.bucketSize).Add(s.bucketSize)
		wait := time.Until(next)
		if wait < 0 {
			wait = time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.fireActive(next)
		}
	}
}

func (s *ForecasterService) fireActive(bucketStart time.Time) {
	s.mu.Lock()
	// Find records matching this bucket boundary across all kinds.
	matching := make([]*ForecastRecord, 0)
	for key, r := range s.activeRecords {
		if r.BucketStart.Equal(bucketStart) {
			matching = append(matching, r)
			delete(s.activeRecords, key)
		} else if r.BucketStart.Before(time.Now().Add(-s.bucketSize)) {
			// Drop stale entries that we never managed to fire.
			delete(s.activeRecords, key)
		}
	}
	s.mu.Unlock()
	for _, r := range matching {
		events.Forecasts.Trigger(events.NewForecastValues().
			SetGeneratedAt(r.GeneratedAt).
			SetModelName(r.ModelName).
			SetKind(events.ForecastKind(r.Kind)).
			SetBucketStart(r.BucketStart).
			SetBucketSize(r.BucketSize).
			SetWatts(r.Watts).
			SetConfidence(r.Confidence).
			SetActive(true))
	}
}

// HistoricalAverageForecaster produces forecasts by averaging the same
// bucket-of-week (load) or bucket-of-day (pv) over the last `weeks` weeks.
type HistoricalAverageForecaster struct {
	system    *System
	repo      Repository
	kind      events.ForecastKind
	modelName string
	weeks     int
}

// NewHistoricalLoadForecaster returns a load forecaster that uses
// rolling weekly averages on grid + pv power, excluding ForecastExclude AC loads.
func NewHistoricalLoadForecaster(system *System, repo Repository, modelName string, weeks int) *HistoricalAverageForecaster {
	if weeks <= 0 {
		weeks = 4
	}
	if modelName == "" {
		modelName = "historical-avg-load"
	}
	return &HistoricalAverageForecaster{system: system, repo: repo, kind: events.ForecastKindLoad, modelName: modelName, weeks: weeks}
}

// NewHistoricalPvForecaster returns a PV forecaster that averages PV
// production over the same bucket-of-day in the last `weeks` weeks.
func NewHistoricalPvForecaster(system *System, repo Repository, modelName string, weeks int) *HistoricalAverageForecaster {
	if weeks <= 0 {
		weeks = 4
	}
	if modelName == "" {
		modelName = "historical-avg-pv"
	}
	return &HistoricalAverageForecaster{system: system, repo: repo, kind: events.ForecastKindPv, modelName: modelName, weeks: weeks}
}

func (h *HistoricalAverageForecaster) Name() string {
	return h.modelName
}

func (h *HistoricalAverageForecaster) Forecast(_ context.Context, from, till time.Time, bucketSize time.Duration) ([]*ForecastRecord, error) {
	hist, err := h.collectHistory(bucketSize)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	bucketsList := buckets.Range(from, till, bucketSize)
	out := make([]*ForecastRecord, 0, len(bucketsList))
	for _, b := range bucketsList {
		key := h.bucketKey(b.Start, bucketSize)
		watts := float32(0)
		confidence := float32(0)
		if values, ok := hist[key]; ok && len(values) > 0 {
			var sum float32
			for _, v := range values {
				sum += v
			}
			watts = sum / float32(len(values))
			confidence = float32(len(values)) / float32(h.weeks)
			if confidence > 1 {
				confidence = 1
			}
		}
		if watts < 0 {
			watts = 0
		}
		out = append(out, &ForecastRecord{
			BucketStart: b.Start,
			BucketSize:  bucketSize,
			GeneratedAt: now,
			ModelName:   h.modelName,
			Kind:        string(h.kind),
			Watts:       watts,
			Confidence:  confidence,
		})
	}
	return out, nil
}

// bucketKey returns a hash key for matching historical buckets.
// Load forecasts use bucket-of-week (so weekday + time of day are preserved);
// PV forecasts use bucket-of-day (so time of day matches across weekdays).
func (h *HistoricalAverageForecaster) bucketKey(t time.Time, bucketSize time.Duration) int {
	// Bucket index relative to the start of the time unit, in bucketSize multiples.
	switch h.kind {
	case events.ForecastKindLoad:
		// Sunday=0..Saturday=6. Combine weekday & time of day.
		dayMinutes := int(t.Weekday())*24*60 + t.Hour()*60 + t.Minute()
		return dayMinutes / int(bucketSize/time.Minute)
	default:
		// PV: bucket-of-day
		dayMinutes := t.Hour()*60 + t.Minute()
		return dayMinutes / int(bucketSize/time.Minute)
	}
}

// collectHistory walks the last N weeks of meter data and groups per bucketKey.
// For load: sum grid + pv − excluded AC loads (mean power per bucket).
// For pv: sum pv (mean power per bucket).
func (h *HistoricalAverageForecaster) collectHistory(bucketSize time.Duration) (map[int][]float32, error) {
	end := time.Now()
	start := end.Add(-time.Duration(h.weeks) * 7 * 24 * time.Hour)
	agg := aggregateConfigForBucket(bucketSize, AggregateFunctionMean)

	out := make(map[int][]float32)

	// PV power records (used by both load and pv kinds).
	pvByBucket := make(map[time.Time]float32)
	for _, pv := range h.system.Pvs() {
		records, err := h.repo.ElectricityStates(start, end, pv.Name(), agg)
		if err != nil {
			log.Debugf("forecaster: pv state lookup failed for %s: %v", pv.Name(), err)
			continue
		}
		for _, r := range records {
			if r.Role != string(constants.EnergySourceRolePv) {
				continue
			}
			s, ok := r.States[AggregateFunctionMean]
			if !ok || s == nil {
				continue
			}
			pvByBucket[r.StartTime] += s.TotalPower()
		}
	}

	if h.kind == events.ForecastKindPv {
		for ts, w := range pvByBucket {
			out[h.bucketKey(ts, bucketSize)] = append(out[h.bucketKey(ts, bucketSize)], w)
		}
		return out, nil
	}

	// Load = grid total + pv total − excluded AC loads
	gridByBucket := make(map[time.Time]float32)
	if h.system.Grid() != nil {
		records, err := h.repo.ElectricityStates(start, end, h.system.Grid().Name(), agg)
		if err == nil {
			for _, r := range records {
				if r.Role != string(constants.EnergySourceRoleGrid) {
					continue
				}
				s, ok := r.States[AggregateFunctionMean]
				if !ok || s == nil {
					continue
				}
				gridByBucket[r.StartTime] += s.TotalPower()
			}
		} else {
			log.Debugf("forecaster: grid state lookup failed: %v", err)
		}
	}

	excludedByBucket := make(map[time.Time]float32)
	for _, acl := range h.system.AcLoads() {
		if !acl.ForecastExclude() {
			continue
		}
		records, err := h.repo.ElectricityStates(start, end, acl.Name(), agg)
		if err != nil {
			log.Debugf("forecaster: ac-load state lookup failed for %s: %v", acl.Name(), err)
			continue
		}
		for _, r := range records {
			s, ok := r.States[AggregateFunctionMean]
			if !ok || s == nil {
				continue
			}
			excludedByBucket[r.StartTime] += s.TotalPower()
		}
	}

	// Combine; iterate over union of timestamps.
	timestamps := make(map[time.Time]struct{})
	for ts := range gridByBucket {
		timestamps[ts] = struct{}{}
	}
	for ts := range pvByBucket {
		timestamps[ts] = struct{}{}
	}
	for ts := range timestamps {
		load := gridByBucket[ts] + pvByBucket[ts] - excludedByBucket[ts]
		if load < 0 {
			load = 0
		}
		out[h.bucketKey(ts, bucketSize)] = append(out[h.bucketKey(ts, bucketSize)], load)
	}
	return out, nil
}

// aggregateConfigForBucket builds an AggregateConfiguration with minute
// granularity matching the given bucket size.
func aggregateConfigForBucket(size time.Duration, functions ...AggregateFunction) *AggregateConfiguration {
	if size <= 0 {
		size = buckets.DefaultBucketSize
	}
	minutes := uint64(size / time.Minute)
	if minutes == 0 {
		minutes = 1
	}
	return &AggregateConfiguration{
		WindowUnit:   WindowUnitMinute,
		WindowAmount: minutes,
		Functions:    functions,
		CreateEmpty:  false,
	}
}

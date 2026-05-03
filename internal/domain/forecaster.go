package domain

import (
	"context"
	"enman/internal/domain/buckets"
	"enman/internal/domain/constants"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/log"
	"sync"
	"time"
)

// Forecaster produces forecast buckets for [from, till) at a given bucket size,
// scoped to a single source (e.g. one PV array, one AC load, one battery).
//
// Implementations must be deterministic and side-effect free; persistence is
// handled by the repository's event listener and event broadcasting by
// ForecasterService.
type Forecaster interface {
	Name() string
	Kind() events.ForecastKind
	SourceName() string
	Forecast(ctx context.Context, from, till time.Time, bucketSize time.Duration, inputs ForecastInputs) ([]*ForecastRecord, error)
}

// ForecastInputs carries already-computed historical forecasts within the same
// runOnce pass, keyed by kind and source name. Used by the price-aware battery
// forecaster to avoid reading half-written data back from the repository.
type ForecastInputs struct {
	ByKind map[events.ForecastKind]map[string][]*ForecastRecord
	Prices []*prices.EnergyPrice
}

// ForecasterService runs all configured forecasters periodically.
type ForecasterService struct {
	system                *System
	repo                  Repository
	historicalForecasters []Forecaster
	batteryForecasters    []Forecaster // run after historical ones in the same pass
	priceProviderName     string
	bucketSize            time.Duration
	horizon               time.Duration
	interval              time.Duration

	mu            sync.Mutex
	activeRecords map[string]*ForecastRecord // key = kind|source|bucket-rfc3339
}

// NewForecasterService creates a new service. bucketSize/horizon/interval must be > 0.
// historicalForecasters run first in each pass; batteryForecasters get the
// resulting per-source records via ForecastInputs and run last.
func NewForecasterService(system *System, repo Repository, bucketSize, horizon, interval time.Duration, priceProviderName string, historicalForecasters []Forecaster, batteryForecasters []Forecaster) *ForecasterService {
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
		system:                system,
		repo:                  repo,
		historicalForecasters: historicalForecasters,
		batteryForecasters:    batteryForecasters,
		priceProviderName:     priceProviderName,
		bucketSize:            bucketSize,
		horizon:               horizon,
		interval:              interval,
		activeRecords:         make(map[string]*ForecastRecord),
	}
}

// Start runs the forecaster loop and a per-bucket "active" event fan-out
// loop. It returns when ctx is cancelled. Suitable for use with errgroup.
func (s *ForecasterService) Start(ctx context.Context) error {
	if len(s.historicalForecasters) == 0 && len(s.batteryForecasters) == 0 {
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

// runOnce generates forecasts for the configured horizon, emits them as events
// (the repository persists them via its event listener), and feeds the latest
// historical forecasts to the battery forecasters in the same pass.
func (s *ForecasterService) runOnce(ctx context.Context) {
	now := time.Now()
	from := buckets.Align(now, s.bucketSize)
	till := from.Add(s.horizon)

	inputs := ForecastInputs{
		ByKind: make(map[events.ForecastKind]map[string][]*ForecastRecord),
	}

	for _, f := range s.historicalForecasters {
		records, err := f.Forecast(ctx, from, till, s.bucketSize, inputs)
		if err != nil {
			log.Warningf("Forecaster %s (%s/%s) failed: %v", f.Name(), f.Kind(), f.SourceName(), err)
			continue
		}
		s.emitAndCache(records, &inputs)
	}

	if len(s.batteryForecasters) > 0 {
		// Provide the price curve once for all battery forecasters.
		if s.priceProviderName != "" {
			priceCurve, err := s.repo.EnergyPrices(from, till, s.priceProviderName, prices.EnergyTypeElectricity)
			if err != nil {
				log.Debugf("battery forecaster: failed to load energy prices for provider %s: %v", s.priceProviderName, err)
			} else {
				inputs.Prices = priceCurve
			}
		}
		for _, f := range s.batteryForecasters {
			records, err := f.Forecast(ctx, from, till, s.bucketSize, inputs)
			if err != nil {
				log.Warningf("Forecaster %s (%s/%s) failed: %v", f.Name(), f.Kind(), f.SourceName(), err)
				continue
			}
			s.emitAndCache(records, &inputs)
		}
	}
}

func (s *ForecasterService) emitAndCache(records []*ForecastRecord, inputs *ForecastInputs) {
	for _, r := range records {
		if r == nil {
			continue
		}
		events.Forecasts.Trigger(events.NewForecastValues().
			SetGeneratedAt(r.GeneratedAt).
			SetModelName(r.ModelName).
			SetKind(events.ForecastKind(r.Kind)).
			SetSourceName(r.SourceName).
			SetBucketStart(r.BucketStart).
			SetBucketSize(r.BucketSize).
			SetWatts(r.Watts).
			SetConfidence(r.Confidence).
			SetActive(false))
	}
	s.cacheActiveRecords(records)
	if inputs != nil {
		for _, r := range records {
			if r == nil {
				continue
			}
			kind := events.ForecastKind(r.Kind)
			bySource, ok := inputs.ByKind[kind]
			if !ok {
				bySource = make(map[string][]*ForecastRecord)
				inputs.ByKind[kind] = bySource
			}
			bySource[r.SourceName] = append(bySource[r.SourceName], r)
		}
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
		key := r.Kind + "|" + r.SourceName + "|" + r.BucketStart.UTC().Format(time.RFC3339Nano)
		if existing, ok := s.activeRecords[key]; !ok || r.GeneratedAt.After(existing.GeneratedAt) {
			s.activeRecords[key] = r
		}
	}
}

// activeLoop wakes at every bucket boundary and fires "active" events for the
// records whose BucketStart matches the current bucket.
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
	matching := make([]*ForecastRecord, 0)
	for key, r := range s.activeRecords {
		if r.BucketStart.Equal(bucketStart) {
			matching = append(matching, r)
			delete(s.activeRecords, key)
		} else if r.BucketStart.Before(time.Now().Add(-s.bucketSize)) {
			delete(s.activeRecords, key)
		}
	}
	s.mu.Unlock()
	for _, r := range matching {
		events.Forecasts.Trigger(events.NewForecastValues().
			SetGeneratedAt(r.GeneratedAt).
			SetModelName(r.ModelName).
			SetKind(events.ForecastKind(r.Kind)).
			SetSourceName(r.SourceName).
			SetBucketStart(r.BucketStart).
			SetBucketSize(r.BucketSize).
			SetWatts(r.Watts).
			SetConfidence(r.Confidence).
			SetActive(true))
	}
}

// =============================================================================
// Historical-average forecaster (per source).
// =============================================================================

// HistoricalKeyMode selects how past samples are grouped when averaging.
type HistoricalKeyMode uint8

const (
	// BucketOfWeek groups by weekday + time of day. Use for things with weekly
	// rhythms (household load, AC loads, gas).
	BucketOfWeek HistoricalKeyMode = iota
	// BucketOfDay groups by time of day only. Use for things that depend
	// mainly on the clock/sun (PV, battery historical baseline).
	BucketOfDay
)

// HistoricalSampleProvider returns historical mean-watts per bucket start time
// for the given query window, used by the historical forecaster to compute an
// average per (bucket-of-week | bucket-of-day) key.
type HistoricalSampleProvider func(start, end time.Time, bucketSize time.Duration) (map[time.Time]float32, error)

// HistoricalAverageForecaster forecasts a single source by averaging the same
// bucket-of-day or bucket-of-week over the last `weeks` weeks.
type HistoricalAverageForecaster struct {
	modelName  string
	kind       events.ForecastKind
	sourceName string
	keyMode    HistoricalKeyMode
	weeks      int
	provider   HistoricalSampleProvider
}

func NewHistoricalAverageForecaster(modelName string, kind events.ForecastKind, sourceName string, keyMode HistoricalKeyMode, weeks int, provider HistoricalSampleProvider) *HistoricalAverageForecaster {
	if weeks <= 0 {
		weeks = 4
	}
	if modelName == "" {
		modelName = "historical-avg"
	}
	return &HistoricalAverageForecaster{
		modelName:  modelName,
		kind:       kind,
		sourceName: sourceName,
		keyMode:    keyMode,
		weeks:      weeks,
		provider:   provider,
	}
}

func (h *HistoricalAverageForecaster) Name() string              { return h.modelName }
func (h *HistoricalAverageForecaster) Kind() events.ForecastKind { return h.kind }
func (h *HistoricalAverageForecaster) SourceName() string        { return h.sourceName }

func (h *HistoricalAverageForecaster) Forecast(_ context.Context, from, till time.Time, bucketSize time.Duration, _ ForecastInputs) ([]*ForecastRecord, error) {
	end := time.Now()
	start := end.Add(-time.Duration(h.weeks) * 7 * 24 * time.Hour)
	samples, err := h.provider(start, end, bucketSize)
	if err != nil {
		return nil, err
	}
	hist := make(map[int][]float32)
	for ts, w := range samples {
		key := h.bucketKey(ts, bucketSize)
		hist[key] = append(hist[key], w)
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
		if confidence < 0.1 {
			log.Debugf("forecaster %s/%s: low confidence (%.2f) at %s", h.kind, h.sourceName, confidence, b.Start)
		}
		out = append(out, &ForecastRecord{
			BucketStart: b.Start,
			BucketSize:  bucketSize,
			GeneratedAt: now,
			ModelName:   h.modelName,
			Kind:        string(h.kind),
			SourceName:  h.sourceName,
			Watts:       watts,
			Confidence:  confidence,
		})
	}
	return out, nil
}

// bucketKey returns a hash key for matching historical buckets according to keyMode.
func (h *HistoricalAverageForecaster) bucketKey(t time.Time, bucketSize time.Duration) int {
	bucketMinutes := int(bucketSize / time.Minute)
	if bucketMinutes <= 0 {
		bucketMinutes = 1
	}
	switch h.keyMode {
	case BucketOfWeek:
		dayMinutes := int(t.Weekday())*24*60 + t.Hour()*60 + t.Minute()
		return dayMinutes / bucketMinutes
	default: // BucketOfDay
		dayMinutes := t.Hour()*60 + t.Minute()
		return dayMinutes / bucketMinutes
	}
}

// =============================================================================
// Sample providers backed by the repository.
// =============================================================================

// PvSampleProvider yields historical PV TotalPower per bucket for one PV array.
func PvSampleProvider(repo Repository, pvName string) HistoricalSampleProvider {
	return func(start, end time.Time, bucketSize time.Duration) (map[time.Time]float32, error) {
		agg := aggregateConfigForBucket(bucketSize, AggregateFunctionMean)
		records, err := repo.ElectricityStates(start, end, pvName, agg)
		if err != nil {
			return nil, err
		}
		out := make(map[time.Time]float32, len(records))
		for _, r := range records {
			if r.Role != string(constants.EnergySourceRolePv) {
				continue
			}
			s, ok := r.States[AggregateFunctionMean]
			if !ok || s == nil {
				continue
			}
			out[r.StartTime] += s.TotalPower()
		}
		return out, nil
	}
}

// AcLoadSampleProvider yields historical TotalPower per bucket for one AC load.
func AcLoadSampleProvider(repo Repository, acLoadName string) HistoricalSampleProvider {
	return func(start, end time.Time, bucketSize time.Duration) (map[time.Time]float32, error) {
		agg := aggregateConfigForBucket(bucketSize, AggregateFunctionMean)
		records, err := repo.ElectricityStates(start, end, acLoadName, agg)
		if err != nil {
			return nil, err
		}
		out := make(map[time.Time]float32, len(records))
		for _, r := range records {
			s, ok := r.States[AggregateFunctionMean]
			if !ok || s == nil {
				continue
			}
			out[r.StartTime] += s.TotalPower()
		}
		return out, nil
	}
}

// BatterySampleProvider yields historical battery TotalPower per bucket
// (signed: + charge, - discharge).
func BatterySampleProvider(repo Repository, batteryName string) HistoricalSampleProvider {
	return func(start, end time.Time, bucketSize time.Duration) (map[time.Time]float32, error) {
		agg := aggregateConfigForBucket(bucketSize, AggregateFunctionMean)
		records, err := repo.ElectricityStates(start, end, batteryName, agg)
		if err != nil {
			return nil, err
		}
		out := make(map[time.Time]float32, len(records))
		for _, r := range records {
			s, ok := r.States[AggregateFunctionMean]
			if !ok || s == nil {
				continue
			}
			out[r.StartTime] += s.TotalPower()
		}
		return out, nil
	}
}

// GasSampleProvider yields historical gas consumption rate per bucket. Gas
// usage is energy-like; rate is approximated as (max - min) over the bucket
// divided by the bucket duration in hours, kept in the meter's native units.
func GasSampleProvider(repo Repository, gasSourceName string) HistoricalSampleProvider {
	return func(start, end time.Time, bucketSize time.Duration) (map[time.Time]float32, error) {
		agg := aggregateConfigForBucket(bucketSize, AggregateFunctionMin, AggregateFunctionMax)
		records, err := repo.GasUsages(start, end, gasSourceName, agg)
		if err != nil {
			return nil, err
		}
		out := make(map[time.Time]float32, len(records))
		hours := float32(bucketSize.Hours())
		if hours <= 0 {
			hours = 0.25
		}
		for _, r := range records {
			minU, hasMin := r.Usages[AggregateFunctionMin]
			maxU, hasMax := r.Usages[AggregateFunctionMax]
			if !hasMin || !hasMax || minU == nil || maxU == nil {
				continue
			}
			delta := maxU.GasConsumed() - minU.GasConsumed()
			if delta < 0 {
				delta = 0
			}
			out[r.StartTime] = float32(delta) / hours
		}
		return out, nil
	}
}

// =============================================================================
// Price-aware battery forecaster.
// =============================================================================

// PriceAwareBatteryForecaster derives a forecast for a single battery by running
// the optimizer over the aggregated load + pv historical forecasts and the
// price curve. It must run after the historical forecasters within the same
// runOnce pass (so ForecastInputs.ByKind is populated).
type PriceAwareBatteryForecaster struct {
	modelName string
	battery   *Battery
	system    *System
	optimizer Optimizer
	cfg       OptimizerConfig
}

func NewPriceAwareBatteryForecaster(modelName string, battery *Battery, system *System, optimizer Optimizer, cfg OptimizerConfig) *PriceAwareBatteryForecaster {
	if modelName == "" {
		modelName = "price-aware-battery"
	}
	return &PriceAwareBatteryForecaster{
		modelName: modelName,
		battery:   battery,
		system:    system,
		optimizer: optimizer,
		cfg:       cfg,
	}
}

func (p *PriceAwareBatteryForecaster) Name() string              { return p.modelName }
func (p *PriceAwareBatteryForecaster) Kind() events.ForecastKind { return events.ForecastKindBattery }
func (p *PriceAwareBatteryForecaster) SourceName() string        { return p.battery.Name() }

func (p *PriceAwareBatteryForecaster) Forecast(ctx context.Context, from, till time.Time, bucketSize time.Duration, inputs ForecastInputs) ([]*ForecastRecord, error) {
	cfg := p.cfg
	cfg.BucketSize = bucketSize
	cfg.Horizon = till.Sub(from)

	loadFcst := mergeForecastsByBucket(inputs.ByKind[events.ForecastKindLoad])
	pvFcst := mergeForecastsByBucket(inputs.ByKind[events.ForecastKindPv])

	records := p.optimizer.Plan(ctx, inputs.Prices, loadFcst, pvFcst, []*Battery{p.battery}, cfg)
	now := time.Now()
	out := make([]*ForecastRecord, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		out = append(out, &ForecastRecord{
			BucketStart: r.BucketStart,
			BucketSize:  r.BucketSize,
			GeneratedAt: now,
			ModelName:   p.modelName,
			Kind:        string(events.ForecastKindBattery),
			SourceName:  p.battery.Name(),
			Watts:       r.PowerW,
			Confidence:  1,
		})
	}
	return out, nil
}

// mergeForecastsByBucket sums Watts across multiple sources of the same kind
// into a single synthetic series. Used to feed the optimizer a single "total
// load" or "total pv" curve from the per-source historical forecasts produced
// earlier in the pass.
func mergeForecastsByBucket(bySource map[string][]*ForecastRecord) []*ForecastRecord {
	if len(bySource) == 0 {
		return nil
	}
	merged := make(map[time.Time]*ForecastRecord)
	for _, records := range bySource {
		for _, r := range records {
			if r == nil {
				continue
			}
			existing, ok := merged[r.BucketStart]
			if !ok {
				cp := *r
				merged[r.BucketStart] = &cp
				continue
			}
			existing.Watts += r.Watts
		}
	}
	out := make([]*ForecastRecord, 0, len(merged))
	for _, r := range merged {
		out = append(out, r)
	}
	return out
}

// =============================================================================
// Helpers
// =============================================================================

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

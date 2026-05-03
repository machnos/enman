package domain

import (
	"context"
	"enman/internal/domain/buckets"
	"enman/internal/domain/prices"
	"sort"
	"time"
)

// BatteryAction enumerates the possible actions the optimizer plans for a
// single bucket. Recorded on BatteryScheduleRecord.Action.
type BatteryAction string

const (
	BatteryActionIdle            BatteryAction = "idle"
	BatteryActionChargeFromGrid  BatteryAction = "charge_from_grid"
	BatteryActionChargeFromPv    BatteryAction = "charge_from_pv"
	BatteryActionDischargeToHome BatteryAction = "discharge_to_home"
	BatteryActionDischargeToGrid BatteryAction = "discharge_to_grid"
	BatteryActionExportOnly      BatteryAction = "export_only"
)

// OptimizerConfig collects all tuneable parameters of the optimizer.
type OptimizerConfig struct {
	BucketSize              time.Duration
	Horizon                 time.Duration
	MinSoC                  float32 // %
	MaxSoC                  float32 // %
	RoundTripEfficiency     float32 // 0..1; defaults to 0.9 if <= 0
	MinArbitrageMargin      float32 // €/kWh net profit to commit a charge/discharge pair
	CycleCostPerKwhAt100Soh float32
	CycleCostPerKwhAt70Soh  float32
}

// Optimizer plans battery actions. Implementations are pure functions of
// their inputs and never mutate them.
type Optimizer interface {
	Plan(ctx context.Context, priceCurve []*prices.EnergyPrice, loadFcst []*ForecastRecord, pvFcst []*ForecastRecord, batteries []*Battery, cfg OptimizerConfig) []*BatteryScheduleRecord
}

// GreedyOptimizer is a first, transparent implementation.
//
// Algorithm:
//  1. For each bucket, compute residual = load − pv (in W) and convert to energy.
//  2. Greedy self-consumption: surplus → charge (up to power & capacity caps),
//     deficit → discharge (up to power & SoC caps).
//  3. Greedy arbitrage: while we can pair the cheapest remaining charge slot
//     with the priciest remaining discharge slot at a profit > minMargin+cycleCost,
//     reserve the pair and update the SoC trajectory.
//
// The optimizer treats all batteries as one virtual aggregated bank for
// planning, then proportionally distributes the resulting actions per battery.
type GreedyOptimizer struct{}

func NewGreedyOptimizer() *GreedyOptimizer { return &GreedyOptimizer{} }

func (o *GreedyOptimizer) Plan(_ context.Context, priceCurve []*prices.EnergyPrice, loadFcst []*ForecastRecord, pvFcst []*ForecastRecord, batteries []*Battery, cfg OptimizerConfig) []*BatteryScheduleRecord {
	if len(batteries) == 0 || cfg.BucketSize <= 0 {
		return nil
	}
	rtEff := cfg.RoundTripEfficiency
	if rtEff <= 0 || rtEff > 1 {
		rtEff = 0.9
	}
	minSoC := cfg.MinSoC
	maxSoC := cfg.MaxSoC
	if maxSoC <= 0 || maxSoC > 100 {
		maxSoC = 95
	}
	if minSoC < 0 {
		minSoC = 0
	}

	// Aggregate battery capabilities.
	totalCapKwh := float32(0)
	totalChargeW := float32(0)
	totalDischargeW := float32(0)
	currentSoCWeighted := float32(0)
	totalSohCap := float32(0)
	for _, b := range batteries {
		c := b.UsableCapacityKwh()
		totalCapKwh += c
		totalChargeW += b.MaxChargePowerW()
		totalDischargeW += b.MaxDischargePowerW()
		soh := float32(100)
		if b.State() != nil && b.State().SoH() > 0 {
			soh = b.State().SoH()
		}
		totalSohCap += soh * c
		soc := float32(0)
		if b.State() != nil {
			soc = b.State().SoC()
		}
		currentSoCWeighted += soc * c
	}
	if totalCapKwh <= 0 {
		return nil
	}
	currentSoC := currentSoCWeighted / totalCapKwh
	avgSoH := totalSohCap / totalCapKwh
	cycleCost := cycleCostPerKwh(avgSoH, cfg.CycleCostPerKwhAt100Soh, cfg.CycleCostPerKwhAt70Soh)

	// Build aligned bucket list by taking the union of forecast buckets
	// constrained to the horizon.
	now := time.Now()
	from := buckets.Align(now, cfg.BucketSize)
	till := from.Add(cfg.Horizon)
	bucketsList := buckets.Range(from, till, cfg.BucketSize)
	if len(bucketsList) == 0 {
		return nil
	}

	loadByBucket := indexForecast(loadFcst)
	pvByBucket := indexForecast(pvFcst)
	priceByBucket := indexPrice(priceCurve, cfg.BucketSize)

	hours := float32(cfg.BucketSize.Hours())
	maxChargeKwh := totalChargeW * hours / 1000.0
	maxDischargeKwh := totalDischargeW * hours / 1000.0
	socKwhMin := totalCapKwh * minSoC / 100.0
	socKwhMax := totalCapKwh * maxSoC / 100.0
	socKwh := totalCapKwh * currentSoC / 100.0
	if socKwh < socKwhMin {
		socKwh = socKwhMin
	}
	if socKwh > socKwhMax {
		socKwh = socKwhMax
	}

	type slot struct {
		bucketStart time.Time
		residualKwh float32 // load - pv (positive = deficit)
		consumption float32
		feedback    float32
		chargeKwh   float32 // signed planned battery delta this bucket (+ charge, − discharge)
		soc         float32 // kWh after this bucket's chargeKwh
		action      BatteryAction
		reason      string
		hasPrice    bool
		hasForecast bool
	}

	slots := make([]slot, len(bucketsList))
	for i, b := range bucketsList {
		s := slot{bucketStart: b.Start, action: BatteryActionIdle}
		if l, ok := loadByBucket[b.Start]; ok {
			s.residualKwh += l * hours / 1000.0
			s.hasForecast = true
		}
		if pv, ok := pvByBucket[b.Start]; ok {
			s.residualKwh -= pv * hours / 1000.0
			s.hasForecast = true
		}
		if p, ok := priceByBucket[b.Start]; ok {
			s.consumption = p.ConsumptionPrice
			s.feedback = p.FeedbackPrice
			s.hasPrice = true
		}
		slots[i] = s
	}

	// Pass 1: greedy self-consumption.
	for i := range slots {
		s := &slots[i]
		if s.residualKwh < 0 {
			// surplus → try to charge
			room := socKwhMax - socKwh
			absorb := -s.residualKwh
			if absorb > maxChargeKwh {
				absorb = maxChargeKwh
			}
			if absorb > room {
				absorb = room
			}
			if absorb > 0 {
				s.chargeKwh = absorb
				socKwh += absorb
				s.action = BatteryActionChargeFromPv
				s.reason = "self-consumption: PV surplus"
			}
		} else if s.residualKwh > 0 {
			// deficit → try to discharge
			avail := socKwh - socKwhMin
			deliver := s.residualKwh
			if deliver > maxDischargeKwh {
				deliver = maxDischargeKwh
			}
			if deliver > avail {
				deliver = avail
			}
			if deliver > 0 {
				s.chargeKwh = -deliver
				socKwh -= deliver
				s.action = BatteryActionDischargeToHome
				s.reason = "self-consumption: cover deficit"
			}
		}
		s.soc = socKwh
	}

	// Pass 2: greedy arbitrage. Sort by price.
	type idx struct{ i int }
	chargeCandidates := make([]int, 0)
	dischargeCandidates := make([]int, 0)
	for i, s := range slots {
		if !s.hasPrice {
			continue
		}
		if s.chargeKwh == 0 {
			chargeCandidates = append(chargeCandidates, i)
			dischargeCandidates = append(dischargeCandidates, i)
		}
	}
	sort.SliceStable(chargeCandidates, func(a, b int) bool {
		return slots[chargeCandidates[a]].consumption < slots[chargeCandidates[b]].consumption
	})
	sort.SliceStable(dischargeCandidates, func(a, b int) bool {
		return slots[dischargeCandidates[a]].feedback > slots[dischargeCandidates[b]].feedback
	})

	// Pair them as long as profitable.
	for _, ci := range chargeCandidates {
		c := &slots[ci]
		if c.chargeKwh != 0 {
			continue
		}
		// Find the best discharge slot strictly after the charge time that
		// is still pristine and profitable.
		for _, di := range dischargeCandidates {
			if di <= ci {
				continue
			}
			d := &slots[di]
			if d.chargeKwh != 0 {
				continue
			}
			profit := d.feedback - c.consumption/rtEff
			if profit <= cfg.MinArbitrageMargin+cycleCost {
				continue
			}
			// How much energy can we move? Limited by charge power, discharge
			// power and remaining capacity headroom over the path.
			pairKwh := maxChargeKwh
			if maxDischargeKwh < pairKwh {
				pairKwh = maxDischargeKwh
			}
			// Capacity headroom along the path between ci and di.
			minHeadroom := socKwhMax - slots[ci].soc
			minSocPath := slots[ci].soc + pairKwh
			for k := ci + 1; k <= di; k++ {
				if k == di {
					break
				}
				room := socKwhMax - slots[k].soc
				if room < minHeadroom {
					minHeadroom = room
				}
				if slots[k].soc+pairKwh < minSocPath {
					minSocPath = slots[k].soc + pairKwh
				}
			}
			if minHeadroom < pairKwh {
				pairKwh = minHeadroom
			}
			if pairKwh <= 0 {
				continue
			}
			// Commit
			c.chargeKwh = pairKwh
			c.action = BatteryActionChargeFromGrid
			c.reason = "arbitrage: cheap charge slot"
			d.chargeKwh = -pairKwh
			d.action = BatteryActionDischargeToGrid
			d.reason = "arbitrage: priced discharge slot"
			// Propagate the SoC change between ci and di (inclusive of di's effect).
			for k := ci; k <= di; k++ {
				if k == ci {
					slots[k].soc += pairKwh
				} else if k == di {
					slots[k].soc = slots[k-1].soc - pairKwh
				} else {
					// keep self-consumption deltas; the absolute SoC shifts by +pairKwh.
					slots[k].soc += pairKwh
				}
			}
			break
		}
	}

	// Convert the per-slot plan into BatteryScheduleRecords for each battery,
	// proportionally distributing the action by power capability.
	now2 := time.Now()
	out := make([]*BatteryScheduleRecord, 0, len(slots)*len(batteries))
	for _, b := range batteries {
		share := float32(0)
		if totalChargeW+totalDischargeW > 0 {
			share = (b.MaxChargePowerW() + b.MaxDischargePowerW()) / (totalChargeW + totalDischargeW)
		}
		for _, s := range slots {
			signedKwh := s.chargeKwh * share
			powerW := signedKwh * 1000.0 / hours
			predictedSoCKwh := s.soc * share
			predictedSoCPct := float32(0)
			cap := b.UsableCapacityKwh()
			if cap > 0 {
				predictedSoCPct = predictedSoCKwh / cap * 100.0
			}
			out = append(out, &BatteryScheduleRecord{
				BucketStart:  s.bucketStart,
				BucketSize:   cfg.BucketSize,
				GeneratedAt:  now2,
				BatteryName:  b.Name(),
				Action:       string(s.action),
				PowerW:       powerW,
				PredictedSoC: predictedSoCPct,
				Reason:       s.reason,
			})
		}
	}
	return out
}

func indexForecast(records []*ForecastRecord) map[time.Time]float32 {
	out := make(map[time.Time]float32, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		out[r.BucketStart] = r.Watts
	}
	return out
}

func indexPrice(curve []*prices.EnergyPrice, bucketSize time.Duration) map[time.Time]*prices.EnergyPrice {
	out := make(map[time.Time]*prices.EnergyPrice, len(curve))
	for _, p := range curve {
		if p == nil {
			continue
		}
		// Snap each price's start time to the bucket grid so look-ups match.
		out[buckets.Align(p.Time, bucketSize)] = p
	}
	return out
}

// cycleCostPerKwh interpolates linearly between the configured 100% and 70%
// SoH endpoints, clamping below 70%.
func cycleCostPerKwh(soh, costAt100, costAt70 float32) float32 {
	if costAt100 <= 0 && costAt70 <= 0 {
		return 0
	}
	if soh >= 100 {
		return costAt100
	}
	if soh <= 70 {
		return costAt70
	}
	// Linear interpolation
	t := (100 - soh) / 30.0
	return costAt100 + (costAt70-costAt100)*t
}

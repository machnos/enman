package domain

import (
	"context"
	"enman/internal/domain/events"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"fmt"
	"math"
	"sync"
	"time"
)

type GridTargetConsumptionCalculator struct {
	system               *System
	roundTripEfficiency  float32
	ticker               *time.Ticker
	meterValues          sync.Map
	lastSetTo            int
	priceBasedCalculator *PriceBasedElectricityConsumptionCalculator
}

func NewGridTargetConsumptionCalculator(system *System, repo repository.Repository, roundTripEfficiency float32) (*GridTargetConsumptionCalculator, error) {
	calculator := &GridTargetConsumptionCalculator{
		system:              system,
		roundTripEfficiency: roundTripEfficiency,
		lastSetTo:           system.Grid().ElectricityTargetConsumption(),
	}
	if system.Grid().controller == nil {
		return nil, fmt.Errorf("no grid controller configured")
	}

	for _, acLoad := range system.AcLoads() {
		if acLoad.PercentageFromGrid() > 0 && acLoad.PercentageFromGrid() <= 100 {
			calculator.meterValues.Store(fmt.Sprintf("%s_%s", acLoad.Name(), acLoad.Role()), &meterData{
				acLoad.percentageFromGrid,
				make([]int, 0),
				sync.Mutex{},
			})
			acLoadCopy := acLoad // Fix Issue #3: Copy loop variable before capturing in closure
			events.ElectricityMeterReadings.Register(calculator, func(values *events.ElectricityMeterValues) bool {
				return acLoadCopy.Name() == values.Name() && acLoadCopy.Role() == values.Role()
			})
		}
	}

	// Collect PV names from the system
	pvNames := make([]string, 0, len(system.Pvs()))
	for _, pv := range system.Pvs() {
		pvNames = append(pvNames, pv.Name())
	}

	calculator.priceBasedCalculator = NewPriceBasedElectricityConsumptionCalculator(
		repo,
		system.Grid().Name(),
		system.Batteries(),
		pvNames,
		roundTripEfficiency,
	)

	return calculator, nil
}

// Start begins the polling loop - returns when context is cancelled (for errgroup integration)
func (g *GridTargetConsumptionCalculator) Start(ctx context.Context) error {
	if g.ticker != nil {
		// Already started
		return nil
	}

	// Start the price-based calculator's background optimization
	if g.priceBasedCalculator != nil {
		g.priceBasedCalculator.Start(ctx)
	}

	g.ticker = time.NewTicker(5 * time.Second)
	defer g.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: reset to initial value
			if g.lastSetTo != g.system.Grid().ElectricityTargetConsumption() {
				err := g.system.Grid().controller.SetElectricityTargetConsumption(g.system.Grid().ElectricityTargetConsumption())
				if err != nil {
					log.Errorf("Failed to reset grid target consumption to initial (configured) value: %v", err)
				}
			}
			return nil
		case <-g.ticker.C:
			addition := g.system.Grid().ElectricityTargetConsumption()
			g.meterValues.Range(func(_, value any) bool {
				data := value.(*meterData)
				addition += data.addition()
				data.reset()
				return true
			})

			// Add price-based battery charging if enabled
			if g.priceBasedCalculator != nil {
				// Calculate available charge power from grid (negative consumption means grid can supply more)
				availablePower := g.system.Grid().MaxElectricityConsumption() - float32(addition)
				priceAddition := g.priceBasedCalculator.CalculateAddition(availablePower)
				addition += int(priceAddition)
			}

			if g.lastSetTo != addition {
				targetConsumption := 0
				if addition >= 0 {
					targetConsumption = min(addition, int(g.system.Grid().MaxElectricityConsumption()))
				} else {
					targetConsumption = max(addition, int(g.system.Grid().MaxElectricityProduction()*-1))
				}
				err := g.system.Grid().controller.SetElectricityTargetConsumption(targetConsumption)
				if err != nil {
					log.Errorf("Failed to set grid target consumption: %v", err)
				} else {
					g.lastSetTo = addition
				}
			}
		}
	}
}

func (g *GridTargetConsumptionCalculator) HandleEvent(values *events.ElectricityMeterValues) {
	valid, err := values.Valid()
	if !valid {
		log.Warningf("Ignoring electricity meter reading from '%s' for grid target consumption calculations as it is invalid: %v", values.Name(), err)
		return
	}
	if values.State().TotalPower() < 0 || values.State().TotalPower() > math.MaxInt16 {
		log.Warningf("Ignoring electricity meter reading from '%s' for grid target consumption calculations as it is > %d or < 0", values.Name(), math.MaxInt16)
		return
	}
	meterKey := fmt.Sprintf("%s_%s", values.Name(), values.Role())
	value, ok := g.meterValues.Load(meterKey)
	if !ok {
		log.Warningf("Unable to calculate grid target consumption because metervalues for '%s' not found", meterKey)
		return
	}
	data := value.(*meterData)
	data.values = append(data.values, int(values.State().TotalPower()))
}

// Stop deregisters event handlers (called during cleanup)
func (g *GridTargetConsumptionCalculator) Stop() {
	events.ElectricityMeterReadings.Deregister(g)
	if g.priceBasedCalculator != nil {
		g.priceBasedCalculator.Stop()
	}
}

type meterData struct {
	percentageFromGrid uint8
	values             []int
	mutex              sync.Mutex
}

func (m *meterData) reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.values = nil
}

func (m *meterData) average() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	total := 0
	if len(m.values) < 1 {
		return total
	}
	for _, number := range m.values {
		total += number
	}
	return total / len(m.values)
}

func (m *meterData) addition() int {
	return (m.average() * int(m.percentageFromGrid)) / 100
}

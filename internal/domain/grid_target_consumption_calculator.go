package domain

import (
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
	ticker               *time.Ticker
	tickerDoneChannel    chan bool
	meterValues          sync.Map
	lastSetTo            int
	priceBasedCalculator *PriceBasedElectricityConsumptionCalculator
}

func NewGridTargetConsumptionCalculator(system *System, repo repository.Repository) (*GridTargetConsumptionCalculator, error) {
	calculator := &GridTargetConsumptionCalculator{
		system:    system,
		lastSetTo: system.Grid().ElectricityTargetConsumption(),
	}
	if system.Grid().controller == nil {
		return nil, fmt.Errorf("no grid controller configured")
	}
	calculator.ticker = time.NewTicker(5 * time.Second)
	calculator.tickerDoneChannel = make(chan bool)

	for _, acLoad := range system.AcLoads() {
		if acLoad.PercentageFromGrid() > 0 && acLoad.PercentageFromGrid() <= 100 {
			calculator.meterValues.Store(fmt.Sprintf("%s_%s", acLoad.Name(), acLoad.Role()), &meterData{
				acLoad.percentageFromGrid,
				make([]int, 0),
				sync.Mutex{},
			})
			events.ElectricityMeterReadings.Register(calculator, func(values *events.ElectricityMeterValues) bool {
				return acLoad.Name() == values.Name() && acLoad.Role() == values.Role()
			})
		}
	}

	calculator.priceBasedCalculator = NewPriceBasedElectricityConsumptionCalculator(repo, system.Grid().Name(), system.Batteries())

	go func() {
		for {
			select {
			case <-calculator.tickerDoneChannel:
				return
			case <-calculator.ticker.C:
				addition := calculator.system.Grid().ElectricityTargetConsumption()
				calculator.meterValues.Range(func(_, value any) bool {
					data := value.(*meterData)
					addition += data.addition()
					data.reset()
					return true
				})

				// Add price-based battery charging if enabled
				if calculator.priceBasedCalculator != nil {
					// Calculate available charge power from grid (negative consumption means grid can supply more)
					availablePower := calculator.system.Grid().MaxElectricityConsumption() - float32(addition)
					priceAddition := calculator.priceBasedCalculator.CalculateAddition(availablePower)
					addition += priceAddition
				}

				if calculator.lastSetTo != addition {
					targetConsumption := 0
					if addition >= 0 {
						targetConsumption = min(addition, int(calculator.system.Grid().MaxElectricityConsumption()))
					} else {
						targetConsumption = max(addition, int(calculator.system.Grid().MaxElectricityProduction()*-1))
					}
					err := calculator.system.Grid().controller.SetElectricityTargetConsumption(targetConsumption)
					if err != nil {
						log.Errorf("Failed to set grid target consumption: %v", err)
					} else {
						calculator.lastSetTo = addition
					}
				}
			}
		}
	}()
	return calculator, nil
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

// EnablePriceBasedCharging enables price-based battery charging by providing
// the energy price repository, the energy provider name, the standard deviation multiplier, and the SoC threshold.
func (g *GridTargetConsumptionCalculator) EnablePriceBasedCharging(repo repository.EnergyPrice, providerName string, stdDevMultiplier float32, socThreshold float32) {
	if g.priceBasedCalculator == nil {
		log.Warningf("Price-based calculator not initialized, cannot enable price-based charging")
		return
	}
	if repo == nil || providerName == "" {
		log.Warningf("Invalid repository or provider name for price-based charging")
		return
	}
	if stdDevMultiplier <= 0 {
		stdDevMultiplier = 1.0
	}
	if socThreshold < 0 || socThreshold > 100 {
		socThreshold = 25.0
	}
	g.priceBasedCalculator = NewPriceBasedElectricityConsumptionCalculator(repo, providerName, g.system.Batteries())
	log.Infof("Price-based battery charging enabled with provider: %s, stdDev multiplier: %.1f, SoC threshold: %.1f%%", providerName, stdDevMultiplier, socThreshold)
}

func (g *GridTargetConsumptionCalculator) Stop() {
	if g.ticker == nil {
		return
	}
	if g.lastSetTo != g.system.Grid().ElectricityTargetConsumption() {
		err := g.system.Grid().controller.SetElectricityTargetConsumption(g.system.grid.ElectricityTargetConsumption())
		if err != nil {
			log.Errorf("Failed to reset grid target consumption to initial (configured) value: %v", err)
		} else {
			g.lastSetTo = g.system.Grid().ElectricityTargetConsumption()
		}
	}
	g.ticker.Stop()
	g.tickerDoneChannel <- true
	for _, acLoad := range g.system.AcLoads() {
		if acLoad.PercentageFromGrid() > 0 {
			events.ElectricityMeterReadings.Deregister(g)
		}
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

package domain

import (
	"enman/internal/log"
	"fmt"
	"math"
	"sync"
	"time"
)

type GridTargetConsumptionCalculator struct {
	system            *System
	ticker            *time.Ticker
	tickerDoneChannel chan bool
	meterValues       sync.Map
	lastSetTo         int
}

func NewGridTargetConsumptionCalculator(system *System) (*GridTargetConsumptionCalculator, error) {
	calculator := &GridTargetConsumptionCalculator{
		system:    system,
		lastSetTo: system.Grid().targetConsumption,
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
			ElectricityMeterReadings.Register(calculator, func(values *ElectricityMeterValues) bool {
				return acLoad.Name() == values.Name() && acLoad.Role() == values.Role()
			})
		}
	}
	go func() {
		for {
			select {
			case <-calculator.tickerDoneChannel:
				return
			case <-calculator.ticker.C:
				addition := calculator.system.Grid().targetConsumption
				calculator.meterValues.Range(func(_, value any) bool {
					data := value.(*meterData)
					addition += data.addition()
					data.reset()
					return true
				})
				if calculator.lastSetTo != addition {
					err := calculator.system.Grid().controller.SetTargetConsumption(addition)
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

func (g *GridTargetConsumptionCalculator) HandleEvent(values *ElectricityMeterValues) {
	valid, err := values.Valid()
	if !valid {
		log.Warningf("Ignoring electricity meter reading from '%s' for grid target consumption calculations as it is invalid: %v", values.Name(), err)
		return
	}
	if values.ElectricityState().TotalPower() < 0 || values.ElectricityState().TotalPower() > math.MaxInt16 {
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
	data.values = append(data.values, int(values.ElectricityState().TotalPower()))
}

func (g *GridTargetConsumptionCalculator) Stop() {
	if g.ticker == nil {
		return
	}
	if g.lastSetTo != g.system.Grid().targetConsumption {
		err := g.system.Grid().controller.SetTargetConsumption(g.system.grid.TargetConsumption())
		if err != nil {
			log.Errorf("Failed to reset grid target consumption to initial (configured) value: %v", err)
		} else {
			g.lastSetTo = g.system.Grid().targetConsumption
		}
	}
	g.ticker.Stop()
	g.tickerDoneChannel <- true
	for _, acLoad := range g.system.AcLoads() {
		if acLoad.PercentageFromGrid() > 0 {
			ElectricityMeterReadings.Deregister(g)
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

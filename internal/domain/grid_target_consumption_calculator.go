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
}

func NewGridTargetConsumptionCalculator(system *System) (*GridTargetConsumptionCalculator, error) {
	calculator := &GridTargetConsumptionCalculator{
		system: system,
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
				err := calculator.system.Grid().controller.SetTargetConsumption(addition)
				if err != nil {
					if log.ErrorEnabled() {
						log.Errorf("Failed to set grid target consumption: %v", err)
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
		if log.WarningEnabled() {
			log.Warningf("Ignoring electricity meter reading from '%s' for grid target consumption calculations as it is invalid: %v", values.Name(), err)
		}
		return
	}
	if values.ElectricityState().TotalPower() < 0 || values.ElectricityState().TotalPower() > math.MaxInt16 {
		if log.WarningEnabled() {
			log.Warningf("Ignoring electricity meter reading from '%s' for grid target consumption calculations as it is > %d or < 0", values.Name(), math.MaxInt16)
		}
		return
	}
	meterKey := fmt.Sprintf("%s_%s", values.Name(), values.Role())
	value, _ := g.meterValues.Load(meterKey)
	data := value.(*meterData)
	data.values = append(data.values, int(values.ElectricityState().TotalPower()))
	//g.meterValues.Store(meterKey, data)
}

func (g *GridTargetConsumptionCalculator) Stop() {
	if g.ticker == nil {
		return
	}
	err := g.system.Grid().controller.SetTargetConsumption(g.system.grid.TargetConsumption())
	if err != nil {
		if log.ErrorEnabled() {
			log.Errorf("Failed to reset grid target consumption to initial (configured) value: %v", err)
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
}

func (m *meterData) reset() {
	m.values = nil
}

func (m *meterData) average() int {
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

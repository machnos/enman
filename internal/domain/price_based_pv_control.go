package domain

import (
	"enman/internal/domain/arithmetic"
	"enman/internal/log"
	"strings"
)

type PriceBasedPVControl struct {
	system *System
}

func NewPriceBasedPVControl(system *System) *PriceBasedPVControl {
	return &PriceBasedPVControl{
		system: system,
	}
}

func (p *PriceBasedPVControl) HandleEvent(values *ElectricityPriceValues) {
	variables := make(map[string]float64)
	variables[values.EnergyProviderName()+".feedback"] = float64(values.FeedbackPrice())
	variables[values.EnergyProviderName()+".consumption"] = float64(values.ConsumptionPrice())
	for _, pv := range p.system.Pvs() {
		if pv.controller != nil && pv.controller.DisableFormula() != "" {
			if !strings.Contains(pv.controller.DisableFormula(), values.EnergyProviderName()) {
				if log.TraceEnabled() {
					log.Tracef("Formula '%s' doesn't contain name of provider '%s'. Skipping pv controller action.", pv.controller.DisableFormula(), values.EnergyProviderName())
				}
				continue
			}
			disable, err := arithmetic.ParseExpression(pv.controller.DisableFormula(), variables)
			if log.DebugEnabled() {
				log.Debugf("Formula: '%s', feedback: %.5f, consumption: %.5f, disable: %t", pv.controller.DisableFormula(), values.FeedbackPrice(), values.ConsumptionPrice(), disable)
			}
			if err != nil {
				if log.ErrorEnabled() {
					log.Errorf("Unable to execute expression '%s': %v. Skipping pv controller action.", pv.controller.DisableFormula(), err)
				}
				continue
			}
			if disable {
				err = pv.controller.DisablePv()
				if err != nil {
					if log.ErrorEnabled() {
						log.Errorf("Failed to disable PV: %v", err)
					}
				}
			} else {
				err = pv.controller.EnablePv()
				if err != nil {
					if log.ErrorEnabled() {
						log.Errorf("Failed to enable PV: %v", err)
					}
				}
			}
		}
	}
}

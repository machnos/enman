package domain

import (
	"context"
	"enman/internal/domain/arithmetic"
	"enman/internal/domain/events"
	"enman/internal/log"
	"strings"
	"time"
)

type PvStateController struct {
	system                   *System
	disableFormula           string
	batteryCutoffPercentage  float32
	batteryRestartPercentage float32
	gridBasedEnabled         bool
	priceBasedEnabled        bool
	priceBasedPVControl      *priceBasedPVControl
	updateTicker             *time.Ticker
}

func NewPvStateController(system *System, disableFormula string, batteryCutoffPercentage float32, batteryRestartPercentage float32) *PvStateController {
	controller := &PvStateController{
		system:                   system,
		disableFormula:           disableFormula,
		batteryCutoffPercentage:  batteryCutoffPercentage,
		batteryRestartPercentage: batteryRestartPercentage,
		gridBasedEnabled:         true,
		priceBasedEnabled:        true,
	}
	controller.priceBasedPVControl = newPriceBasedPVControl(controller)
	return controller
}

func (p *PvStateController) detectGridBasedPvProduction() {
	if p.system.Grid().controller == nil {
		p.gridBasedEnabled = true
		return
	}
	gridConnected, err := p.system.Grid().controller.GridConnected()
	if err != nil {
		log.Warningf("Failed to determine if grid is connected: %v. Grid loss protection impossible, assuming the grid is lost.", err)
		gridConnected = false
	}
	if gridConnected {
		if !p.gridBasedEnabled {
			log.Infof("Grid is connected. Pv production based on grid connection will be enabled.")
		}
		p.gridBasedEnabled = true
		return
	}
	// We don't have a grid connection at this point
	if len(p.system.Batteries()) < 1 {
		if p.gridBasedEnabled {
			log.Info("No batteries found and grid is lost. Pv production based on grid connection will be disabled.")
		}
		p.gridBasedEnabled = false
		return
	}
	// If any of the batteries is below the enabled threshold SoC we consider oversupply possible.
	// If all the batteries are over the disabled threshold SoC we consider oversupply impossible.
	allOverThreshold := true
	for _, battery := range p.system.Batteries() {
		if battery.State().SoC() < p.batteryRestartPercentage {
			if !p.gridBasedEnabled {
				log.Infof("Grid is lost but battery below %.0f SoC detected. Pv production based on grid connection will be enabled.", p.batteryRestartPercentage)
			}
			p.gridBasedEnabled = true
			return
		} else if battery.State().SoC() < p.batteryCutoffPercentage {
			allOverThreshold = false
			break
		}
	}
	if allOverThreshold {
		if p.gridBasedEnabled {
			log.Infof("Grid is lost and all batteries are over %.0f SoC. Pv production based on grid connection will be disabled.", p.batteryCutoffPercentage)
		}
		p.gridBasedEnabled = false
		return
	}
	// Grid is lost, but not all batteries are over their threshold.
	// Do nothing at this point. Batteries are charged until their all above p.batteryCutoffPercentage
}

func (p *PvStateController) Start(context context.Context) {
	if p.updateTicker != nil {
		return
	}
	events.EnergyPrices.Register(p.priceBasedPVControl, func(priceValues *events.EnergyPriceValues) bool {
		return true
	})
	if p.system.Grid().controller == nil {
		log.Warningf("No grid controller configured. Grid loss protection not possible.")
	}
	if p.disableFormula == "" {
		log.Warningf("No Pvs disable formula configured. Price based PV control not possible.")
	}
	p.updateTicker = time.NewTicker(time.Millisecond * 2500)
	go func() {
		for {
			select {
			case <-context.Done():
				p.updateTicker.Stop()
				events.EnergyPrices.Deregister(p.priceBasedPVControl)
				return
			case _ = <-p.updateTicker.C:
				p.detectGridBasedPvProduction()
				enabled := p.priceBasedEnabled && p.gridBasedEnabled
				for _, pv := range p.system.Pvs() {
					if pv.controller != nil {
						if enabled {
							err := pv.controller.EnablePv()
							if err != nil {
								log.Errorf("Unable to enable pv %s: %v", pv.Name(), err)
							}
						} else {
							err := pv.controller.DisablePv()
							if err != nil {
								log.Errorf("Unable to disable pv %s: %v", pv.Name(), err)
							}
						}
					}
				}
			}
		}
	}()
}

type priceBasedPVControl struct {
	*PvStateController
}

func newPriceBasedPVControl(controller *PvStateController) *priceBasedPVControl {
	return &priceBasedPVControl{
		controller,
	}
}

func (pbc *priceBasedPVControl) HandleEvent(values *events.EnergyPriceValues) {
	if pbc.disableFormula == "" {
		return
	}
	variables := make(map[string]float64)
	variables[values.EnergyProviderName()+".feedback"] = float64(values.FeedbackPrice())
	variables[values.EnergyProviderName()+".consumption"] = float64(values.ConsumptionPrice())
	if !strings.Contains(pbc.disableFormula, values.EnergyProviderName()) {
		log.Tracef("Formula '%s' doesn't contain name of provider '%s'. Skipping pv controller action.", pbc.disableFormula, values.EnergyProviderName())
		return
	}
	disable, err := arithmetic.ParseExpression(pbc.disableFormula, variables)
	if err != nil {
		log.Errorf("Unable to execute expression '%s': %v. Skipping pv controller action.", pbc.disableFormula, err)
		return
	}
	if disable {
		if pbc.priceBasedEnabled {
			log.Infof("Electricity price below threshold. Pv production based on price will be disabled.")
		}
		pbc.priceBasedEnabled = false
	} else {
		if !pbc.priceBasedEnabled {
			log.Infof("Electricity price above threshold. Pv production based on price will be enabled.")
		}
		pbc.priceBasedEnabled = true
	}
}

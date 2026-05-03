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
	// If any of the batteries is below the enable threshold SoC we consider oversupply possible.
	// If all the batteries are over the disable threshold SoC we consider oversupply impossible.
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
		log.Info("No Pvs disable formula configured. Using default rule: disable PV when feedback < 0 and battery cannot absorb; force-disable when feedback < 0 and consumption < 0.")
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
	canAbsorb, avgSoH := pbc.batteryAbsorbState()

	if pbc.disableFormula == "" {
		// Default rule: disable PV when feedback < 0 and we can't absorb the
		// surplus into the battery; force-disable when both feedback and
		// consumption prices are negative (we earn by importing).
		disable := false
		reason := ""
		if values.FeedbackPrice() < 0 && values.ConsumptionPrice() < 0 {
			disable = true
			reason = "feedback and consumption prices both negative"
		} else if values.FeedbackPrice() < 0 && !canAbsorb {
			disable = true
			reason = "feedback price negative and battery cannot absorb"
		}
		pbc.applyPriceDecision(disable, reason)
		return
	}

	variables := make(map[string]float64)
	variables[values.EnergyProviderName()+".feedback"] = float64(values.FeedbackPrice())
	variables[values.EnergyProviderName()+".consumption"] = float64(values.ConsumptionPrice())
	// Generic helpers usable across providers.
	variables["feedback"] = float64(values.FeedbackPrice())
	variables["consumption"] = float64(values.ConsumptionPrice())
	variables["battery.canAbsorb"] = boolToFloat(canAbsorb)
	variables["battery.soh"] = float64(avgSoH)

	if !strings.Contains(pbc.disableFormula, values.EnergyProviderName()) &&
		!strings.Contains(pbc.disableFormula, "feedback") &&
		!strings.Contains(pbc.disableFormula, "consumption") &&
		!strings.Contains(pbc.disableFormula, "battery.") {
		log.Tracef("Formula '%s' doesn't reference provider '%s' or generic vars. Skipping pv controller action.", pbc.disableFormula, values.EnergyProviderName())
		return
	}
	disable, err := arithmetic.ParseExpression(pbc.disableFormula, variables)
	if err != nil {
		log.Errorf("Unable to execute expression '%s': %v. Skipping pv controller action.", pbc.disableFormula, err)
		return
	}
	pbc.applyPriceDecision(disable, "formula '"+pbc.disableFormula+"'")
}

// batteryAbsorbState returns whether any battery currently has room
// (SoC < cutoff) and the average SoH across batteries.
func (pbc *priceBasedPVControl) batteryAbsorbState() (canAbsorb bool, avgSoH float32) {
	batteries := pbc.system.Batteries()
	if len(batteries) == 0 {
		// No batteries: any surplus would have to go to the grid; treat as cannot absorb.
		return false, 0
	}
	totalSoH := float32(0)
	count := float32(0)
	for _, b := range batteries {
		state := b.State()
		if state == nil {
			continue
		}
		if state.SoC() < pbc.batteryCutoffPercentage {
			canAbsorb = true
		}
		totalSoH += state.SoH()
		count++
	}
	if count > 0 {
		avgSoH = totalSoH / count
	}
	return
}

func (pbc *priceBasedPVControl) applyPriceDecision(disable bool, reason string) {
	if disable {
		if pbc.priceBasedEnabled {
			log.Infof("Pv production disabled by price-based control: %s.", reason)
		}
		pbc.priceBasedEnabled = false
	} else {
		if !pbc.priceBasedEnabled {
			log.Infof("Pv production re-enabled by price-based control.")
		}
		pbc.priceBasedEnabled = true
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

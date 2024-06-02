package controllers

import (
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
)

func probeHttpPvController(config *config.PvController) domain.PvController {
	//var controller domain.PvController
	if config.Brand == "Shelly" || config.Brand == "" {
		// Shelly pv controller
		if log.InfoEnabled() {
			log.Infof("Probing for Shelly PV Controller at %s", config.ConnectURL)
		}
		controller, err := newShellyPvController(config)
		if err == nil {
			return controller
		}
		log.Infof("Probe failed for Shelly PV Controller: %v", err)
	}
	log.Warningf("Unable to detect http PV Controller at url '%s'", config.ConnectURL)
	return nil
}

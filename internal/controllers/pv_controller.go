package controllers

import (
	"enman/internal/config"
	"enman/internal/domain"
)

func ProbePvController(config *config.PvController) domain.PvController {
	if config == nil {
		return nil
	}
	if "http" == config.Type {
		return probeHttpPvController(config)
	}
	return nil
}

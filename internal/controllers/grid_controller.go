package controllers

import (
	"enman/internal/config"
	"enman/internal/domain"
)

func ProbeGridController(config *config.GridController) domain.GridController {
	if config == nil {
		return nil
	}
	if "modbus" == config.Type {
		return probeModbusGridController(config)
	}
	return nil
}

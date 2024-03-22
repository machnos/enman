package controllers

import (
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
	"strings"
	"time"
)

func probeModbusGridController(config *config.GridController) domain.GridController {
	var controller domain.GridController
	if strings.HasPrefix(config.ConnectURL, "rtu") {
		probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
		if config.Speed != 0 {
			probeBaudRates = []uint{uint(config.Speed)}
		}
		for _, rate := range probeBaudRates {
			clientConfig := &modbus.ClientConfiguration{
				URL:     config.ConnectURL,
				Timeout: time.Millisecond * 500,
				Speed:   rate,
			}
			modbusClient, clientCached, err := modbus.GetOrCreateCached(clientConfig)
			if err != nil {
				if log.DebugEnabled() {
					log.Debugf("Unable to create modbus client: %v", err)
				}
				continue
			}
			controller = probeGridControllerWithClient(config, modbusClient)
			if controller != nil {
				break
			} else if !clientCached {
				modbus.RemoveCached(modbusClient)
			}
		}
	} else {
		clientConfig := &modbus.ClientConfiguration{
			URL: config.ConnectURL,
		}
		modbusClient, clientCached, err := modbus.GetOrCreateCached(clientConfig)
		if err != nil {
			if log.DebugEnabled() {
				log.Debugf("Unable to create modbus client: %v", err)
			}
		} else {
			controller = probeGridControllerWithClient(config, modbusClient)
			if controller == nil && !clientCached {
				modbus.RemoveCached(modbusClient)
			}
		}
	}
	if controller == nil {
		log.Warningf("Unable to detect modbus grid controller at url '%s'", config.ConnectURL)
	}
	return controller
}

func probeGridControllerWithClient(config *config.GridController, modbusClient *modbus.ModbusClient) domain.GridController {
	if config.Brand == "Victron" || config.Brand == "" {
		// Victron grid controller
		if log.InfoEnabled() {
			baudRateLogging := ""
			if modbusClient.Speed() > 0 {
				baudRateLogging = fmt.Sprintf("baud rate %d and ", modbusClient.Speed())
			}
			log.Infof("Probing for Victron grid controller at %s", baudRateLogging, modbusClient.URL())
		}
		controller, err := newVictronGridController(modbusClient)
		if err == nil {
			return controller
		}
		log.Infof("Probe failed for Victron grid controller: %v", err)
	}
	return nil
}

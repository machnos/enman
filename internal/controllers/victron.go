package controllers

import (
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/modbus"
	"fmt"
)

type victronGridController struct {
	systemUnitId uint8
	modbusClient *modbus.ModbusClient
}

func newVictronGridController(modbusClient *modbus.ModbusClient) (domain.GridController, error) {
	vgm := &victronGridController{
		100,
		modbusClient,
	}
	return vgm, vgm.validGridController()
}

func (v *victronGridController) validGridController() error {
	bytes, err := v.modbusClient.ReadBytes(v.systemUnitId, 800, 6, modbus.INPUT_REGISTER)
	if err != nil {
		return err
	}
	if len(bytes) != 6 {
		return fmt.Errorf("detected an unsupported Victron Grid Controller")
	}
	log.Infof("Detected a Victron Grid Controller at %s.", v.modbusClient.URL())
	return nil
}

func (v *victronGridController) SetTargetConsumption(targetConsumption uint8) error {
	return v.modbusClient.WriteRegister(v.systemUnitId, 2700, uint16(targetConsumption), modbus.BIG_ENDIAN)
}

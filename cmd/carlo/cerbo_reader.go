package main

import (
	"enman/internal/modbus"
	"fmt"
	"syscall"
)

func main() {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL: "tcp://einstein.energy.cleme:502",
	})
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	err = client.Open()
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	unitIdBattery := uint8(225)
	uint16s, err := client.ReadRegisters(unitIdBattery, 258, 2, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)

	if err != nil {
		return
	}
	fmt.Printf("Watts: %.0fw\n", client.ValueFromInt16sResultArray(uint16s, 0, 0, 0))
	fmt.Printf("Voltage: %.2fv\n", client.ValueFromUint16sResultArray(uint16s, 1, 100, 0))

	uint16s, err = client.ReadRegisters(unitIdBattery, 266, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("SoC: %.0f%%\n", client.ValueFromUint16sResultArray(uint16s, 0, 10, 0))

	uint16s, err = client.ReadRegisters(unitIdBattery, 261, 2, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Current: %.1fA\n", client.ValueFromInt16sResultArray(uint16s, 0, 10, 0))
	fmt.Printf("Temperature: %.1fC\n", client.ValueFromInt16sResultArray(uint16s, 1, 10, 0))

	unitIdSystem := uint8(100)
	uint16s, err = client.ReadRegisters(unitIdSystem, 2700, 1, modbus.BIG_ENDIAN, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Grid setpoint: %.0fw\n", client.ValueFromInt16sResultArray(uint16s, 0, 0, 0))

	gridSetPoint := int16(20)
	fmt.Printf("Setting grid setpoint to: %d\n", gridSetPoint)
	err = client.WriteRegister(unitIdSystem, 2700, uint16(gridSetPoint), modbus.BIG_ENDIAN)
	if err != nil {
		fmt.Printf("%s", err.Error())
	}

}

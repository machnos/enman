package main

import (
	"enman/internal/modbus"
	"syscall"
)

func main() {
	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:   "rtu:///dev/ttyUSB0",
		Speed: 9600,
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
	client.SetUnitId(2)
	values, err := client.ReadRegisters(0xb, 1, modbus.INPUT_REGISTER)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	println("EM24 model: ", values[0])

	values, err = client.ReadRegisters(0x1101, 1, modbus.INPUT_REGISTER)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	println("Application type: ", values[0])

	values, err = client.ReadRegisters(0x1100, 1, modbus.INPUT_REGISTER)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	println("Password: ", values[0])

}

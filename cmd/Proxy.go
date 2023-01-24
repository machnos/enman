package main

import (
	"enman/internal/log"
	"enman/internal/modbus"
	"syscall"
	"time"
)

func main() {
	log.ActiveLevel = log.LvlDebug
	cgClient, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "rtu:///dev/ttyUSB3",
		Speed:   9600,
		Timeout: time.Millisecond * 500,
	})

	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	err = cgClient.Open()
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	defer cgClient.Close()

	handler := modbus.NewProxyRequestHandler(cgClient)
	server, err := modbus.NewServer(&modbus.ServerConfiguration{
		URL:   "rtu:///dev/ttyUSB0",
		Speed: 9600,
	}, handler)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	err = server.Start()
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	defer server.Stop()

	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:     "rtu:///dev/ttyUSB1",
		Speed:   9600,
		Timeout: time.Millisecond * 500,
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
	defer client.Close()

	registers, err := client.ReadRegisters(0, 5, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	println(registers[0])

	time.Sleep(10 * time.Second)
}

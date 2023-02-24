package main

import (
	"enman/internal/modbus"
	"fmt"
	"syscall"
	"time"
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
	unitId := uint8(2)
	client.SetEncoding(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST)
	readUint32, err := client.ReadUint32(unitId, 0x000c, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Big, LWF, %d, %d\n", readUint32, int32(readUint32))

	client.SetEncoding(modbus.LITTLE_ENDIAN, modbus.LOW_WORD_FIRST)
	readUint32, err = client.ReadUint32(unitId, 0x000c, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Little, LWF, %d, %d\n", readUint32, int32(readUint32))

	client.SetEncoding(modbus.BIG_ENDIAN, modbus.HIGH_WORD_FIRST)
	readUint32, err = client.ReadUint32(unitId, 0x000c, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Big, HWF, %d, %d\n", readUint32, int32(readUint32))

	client.SetEncoding(modbus.LITTLE_ENDIAN, modbus.HIGH_WORD_FIRST)
	readUint32, err = client.ReadUint32(unitId, 0x000c, modbus.INPUT_REGISTER)
	if err != nil {
		return
	}
	fmt.Printf("Little, HWF, %d, %d\n", readUint32, int32(readUint32))

	values, err := client.ReadRegisters(unitId, 0xb, 1, modbus.INPUT_REGISTER)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	println("EM24 model: ", values[0])

	values, err = client.ReadRegisters(unitId, 0x1101, 1, modbus.INPUT_REGISTER)
	if err != nil {
		println(err.Error())
		syscall.Exit(-1)
	}
	println("Application type: ", values[0])

	for ix := 0; ix < 10; ix++ {
		values, err = client.ReadRegisters(unitId, 0x003e, 1, modbus.INPUT_REGISTER)
		if err != nil {
			println(err.Error())
			syscall.Exit(-1)
		}
		kwhTotPlus := float32(values[0]) / 10
		values, err = client.ReadRegisters(unitId, 0x005c, 1, modbus.INPUT_REGISTER)
		if err != nil {
			println(err.Error())
			syscall.Exit(-1)
		}
		kwhTotMin := float32(values[0]) / 10
		fmt.Printf("kwh+ tot: %4.2f, kwh(-) tot: %4.2f\n", kwhTotPlus, kwhTotMin)
		time.Sleep(time.Second * 1)
	}

}

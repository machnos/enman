package dsmr

import (
	"bufio"
	"enman/internal/energysource"
	"enman/internal/serial"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type DsmrConfig struct {
	Device   string
	BaudRate uint32
}

func NewDsmrGrid(name string, config *DsmrConfig, updateChannel chan energysource.Grid, gridConfig *energysource.GridConfig) (*energysource.GridBase, error) {
	gb := energysource.NewGridBase(name, gridConfig)
	serialPort, err := serial.Open(&serial.Config{
		Address:  config.Device,
		BaudRate: int(config.BaudRate),
		Timeout:  time.Millisecond * 500,
		DataBits: 8,
		Parity:   "N",
		StopBits: 1,
	})
	if err != nil {
		return nil, err
	}
	// TODO log initialization of system. Print meter serial + port that is used.
	go readSystemValues(serialPort, gb, updateChannel)
	return gb, nil
}

func readSystemValues(serialPort serial.Port, grid *energysource.GridBase, updateChannel chan energysource.Grid) {
	tickerChannel := make(chan bool)
	runtime.SetFinalizer(grid, func(grid *energysource.GridBase) {
		tickerChannel <- true
	})
	defer func(serialPort serial.Port) {
		_ = serialPort.Close()
	}(serialPort)

	reader := bufio.NewReader(serialPort)
	for {

		// telegram data is suffixed with a CRC code
		// this CRC code starts with ! so let's read until we receive that char
		//if <-tickerChannel {
		//	break
		//}
		message, err := reader.ReadString('\x21') // hex char code for !
		if err != nil {
			continue
		}
		changed := false
		totalEnergyConsumed := float64(0)
		totalEnergyProvided := float64(0)
		lines := strings.Split(message, "\n")
		for ix := 0; ix < len(lines); ix++ {
			trimmedLine := strings.TrimSpace(lines[ix])
			// TODO test for 1.8.0 -> should be total consumed
			if strings.HasPrefix(trimmedLine, "1-0:1.8.1.255") {
				totalEnergyConsumed += float64(ValueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:1.8.2.255") {
				totalEnergyConsumed += float64(ValueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.1.255") {
				// TODO test for 2.8.0 -> should be total provided
				totalEnergyProvided += float64(ValueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.2.255") {
				totalEnergyProvided += float64(ValueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:32.7.0") {
				valueChanged, _ := grid.SetVoltage(0, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:52.7.0") {
				valueChanged, _ := grid.SetVoltage(1, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:72.7.0") {
				valueChanged, _ := grid.SetVoltage(2, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:31.7.0") {
				valueChanged, _ := grid.SetCurrent(0, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:51.7.0") {
				valueChanged, _ := grid.SetCurrent(1, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:71.7.0") {
				valueChanged, _ := grid.SetCurrent(2, ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:21.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(0, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:41.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(1, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:61.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(2, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:22.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(0, value*-1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:42.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(1, value*-1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:62.7.0") {
				value := ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := grid.SetPower(2, value*-1000)
					changed = changed || valueChanged
				}
			}
		}
		// TODO add KWH per phase, see https://www.netbeheernederland.nl/_upload/Files/Slimme_meter_15_a727fce1f1.pdf
		grid.SetTotalEnergyConsumed(totalEnergyConsumed)
		grid.SetTotalEnergyProvided(totalEnergyProvided)
		if changed && updateChannel != nil {
			updateChannel <- grid
		}
	}
}

func ValueFromObisLine(obisLine string) float32 {
	// TODO waarde kan soms unparsable zijn, bijv 1-0:31.7.0(kW)
	value := obisLine[strings.Index(obisLine, "(")+1 : strings.Index(obisLine, "*")]
	float, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0
	}
	return float32(float)
}

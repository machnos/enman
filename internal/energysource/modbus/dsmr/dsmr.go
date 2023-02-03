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
	Name     string
	Device   string
	BaudRate uint32
}

func NewDsmrGrid(config *DsmrConfig, updateChannel chan energysource.Grid, gridConfig *energysource.GridConfig) (*energysource.GridBase, error) {
	gb := energysource.NewGridBase(config.Name, gridConfig)
	serialPort, err := serial.Open(&serial.Config{
		Address:  config.Device,
		BaudRate: 115200,
		Timeout:  time.Millisecond * 500,
		DataBits: 8,
		Parity:   "N",
		StopBits: 1,
	})
	if err != nil {
		return nil, err
	}
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
		totalEnergyConsumed := float32(0)
		totalEnergyProvided := float32(0)
		lines := strings.Split(message, "\n")
		for ix := 0; ix < len(lines); ix++ {
			trimmedLine := strings.TrimSpace(lines[ix])
			if strings.HasPrefix(trimmedLine, "1-0:1.8.1.255") {
				totalEnergyConsumed += ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:1.8.2.255") {
				totalEnergyConsumed += ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.1.255") {
				totalEnergyProvided += ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.2.255") {
				totalEnergyProvided += ValueFromObisLine(trimmedLine) * 1000
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

		// No option to read the totals per phase, so spread them over the phases evenly.
		consumedPerPhase := totalEnergyConsumed / float32(grid.Phases())
		providedPerPhase := totalEnergyConsumed / float32(grid.Phases())
		for ix := uint8(0); ix < grid.Phases(); ix++ {
			_, _ = grid.SetEnergyConsumed(ix, consumedPerPhase)
			_, _ = grid.SetEnergyProvided(ix, providedPerPhase)
		}
		if changed && updateChannel != nil {
			updateChannel <- grid
		}
	}
}

func ValueFromObisLine(obisLine string) float32 {
	value := obisLine[strings.Index(obisLine, "(")+1 : strings.Index(obisLine, "*")]
	float, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0
	}
	return float32(float)
}

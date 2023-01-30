package energysource

import (
	"bufio"
	"enman/internal/serial"
	"enman/pkg/energysource"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type DsmrConfig struct {
	Device   string
	BaudRate uint32
}

type dsmrSystem struct {
}

func NewDsmrSystem(config *DsmrConfig, gridConfig *energysource.GridConfig) (*energysource.System, error) {
	gb := energysource.NewGridBase(gridConfig)
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
	dSystem := &dsmrSystem{}
	e := energysource.Grid(gb)
	var system = energysource.NewSystem(e, nil)
	go dSystem.readSystemValues(serialPort, system)
	return system, nil
}

func (d *dsmrSystem) readSystemValues(serialPort serial.Port, system *energysource.System) {
	tickerChannel := make(chan bool)
	runtime.SetFinalizer(system, func(system *energysource.System) {
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
		gridBase, ok := system.Grid().(*energysource.GridBase)
		if !ok {
			continue
		}
		changed := false
		totalEnergyConsumed := float32(0)
		totalEnergyProvided := float32(0)
		lines := strings.Split(message, "\n")
		for ix := 0; ix < len(lines); ix++ {
			trimmedLine := strings.TrimSpace(lines[ix])
			if strings.HasPrefix(trimmedLine, "1-0:1.8.1.255") {
				totalEnergyConsumed += d.ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:1.8.2.255") {
				totalEnergyConsumed += d.ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.1.255") {
				totalEnergyProvided += d.ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.2.255") {
				totalEnergyProvided += d.ValueFromObisLine(trimmedLine) * 1000
			} else if strings.HasPrefix(trimmedLine, "1-0:32.7.0") {
				valueChanged, _ := gridBase.SetVoltage(0, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:52.7.0") {
				valueChanged, _ := gridBase.SetVoltage(1, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:72.7.0") {
				valueChanged, _ := gridBase.SetVoltage(2, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:31.7.0") {
				valueChanged, _ := gridBase.SetCurrent(0, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:51.7.0") {
				valueChanged, _ := gridBase.SetCurrent(1, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:71.7.0") {
				valueChanged, _ := gridBase.SetCurrent(2, d.ValueFromObisLine(trimmedLine))
				changed = changed || valueChanged
			} else if strings.HasPrefix(trimmedLine, "1-0:21.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(0, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:41.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(1, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:61.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(2, value*1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:22.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(0, value*-1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:42.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(1, value*-1000)
					changed = changed || valueChanged
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:62.7.0") {
				value := d.ValueFromObisLine(trimmedLine)
				if value > 0 {
					valueChanged, _ := gridBase.SetPower(2, value*-1000)
					changed = changed || valueChanged
				}
			}
		}

		// No option to read the totals per phase, so spread them over the phases evenly.
		consumedPerPhase := totalEnergyConsumed / float32(gridBase.Phases())
		providedPerPhase := totalEnergyConsumed / float32(gridBase.Phases())
		for ix := uint8(0); ix < gridBase.Phases(); ix++ {
			_, _ = gridBase.SetEnergyConsumed(ix, consumedPerPhase)
			_, _ = gridBase.SetEnergyProvided(ix, providedPerPhase)
		}
		if changed && system.LoadUpdated() != nil {
			system.LoadUpdated() <- true
		}
	}
}

func (d *dsmrSystem) ValueFromObisLine(obisLine string) float32 {
	value := obisLine[strings.Index(obisLine, "(")+1 : strings.Index(obisLine, "*")]
	float, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0
	}
	return float32(float)
}

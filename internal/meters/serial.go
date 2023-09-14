package meters

import (
	"bufio"
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/serial"
	"sync"
	"time"
)

type electricitySerialMeter struct {
	genericElectricityMeter
	url            string
	probeWaitGroup sync.WaitGroup
	serialPort     serial.Port
	readValues     func(*electricitySerialMeter, *domain.ElectricityState, *domain.ElectricityUsage)
	reader         *bufio.Reader
}

func NewElectricitySerialMeter(name string, role domain.ElectricitySourceRole, brand string, speed uint16, url string, lineIndices []uint8, attributes string) domain.ElectricityMeter {
	esm := &electricitySerialMeter{
		genericElectricityMeter: genericElectricityMeter{
			name:        name,
			role:        role,
			lineIndices: lineIndices,
			attributes:  attributes,
		},
		url: url,
	}
	probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
	if speed != 0 {
		probeBaudRates = []uint{uint(speed)}
	}

	esm.probeWaitGroup.Add(1)
	go esm.probeMeter(probeBaudRates, brand)
	return esm
}

func (esm *electricitySerialMeter) WaitForInitialization() bool {
	esm.probeWaitGroup.Wait()
	return esm.serialPort != nil
}

func (esm *electricitySerialMeter) StartReading(ctx context.Context) domain.ElectricityMeter {
	if !esm.WaitForInitialization() {
		log.Warningf("Unable to read values from electricity meter %s in role %v as it is an unknown device", esm.name, esm.role)
		return esm
	}
	state := domain.NewElectricityState()
	usage := domain.NewElectricityUsage()

	go func() {
		for {
			select {
			case <-ctx.Done():
				esm.shutdown()
				return
			default:
				esm.readValues(esm, state, usage)
				event := domain.NewElectricityMeterValues().
					SetName(esm.Name()).
					SetRole(esm.Role()).
					SetMeterPhases(esm.Phases()).
					SetElectricityState(state).
					SetElectricityUsage(usage)
				domain.ElectricityMeterReadings.Trigger(event)
			}
		}
	}()
	return esm
}

func (esm *electricitySerialMeter) probeMeter(baudRates []uint, brand string) bool {
	defer esm.probeWaitGroup.Done()
	for _, rate := range baudRates {
		serialPort, err := serial.Open(&serial.Config{
			Address:  esm.url,
			BaudRate: int(rate),
			Timeout:  time.Millisecond * 500,
			DataBits: 8,
			Parity:   "N",
			StopBits: 1,
		})
		if err != nil {
			return false
		}
		if brand == "DSMR" || brand == "" {
			if log.InfoEnabled() {
				log.Infof("Probing for DSMR meter with baud rate %d at %s", rate, esm.url)
			}
			d := &dsmrGridMeter{}
			if d.probe(esm, serialPort) {
				return true
			}
		}
		_ = serialPort.Close()
	}
	return false
}

func (esm *electricitySerialMeter) shutdown() {
	if esm.serialPort != nil {
		log.Infof("Stop reading values from electricity meter %s in role %v", esm.name, esm.role)
		err := esm.serialPort.Close()
		if err != nil && log.DebugEnabled() {
			log.Debugf("Unable to close serial port: %v", err)
		}
		esm.serialPort = nil
	}
}

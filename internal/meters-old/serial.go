package meters_old

import (
	"bufio"
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/serial"
	"sync"
	"time"
)

type serialElectricityMeter struct {
	url            string
	probeWaitGroup sync.WaitGroup
	serialPort     serial.Port
	readValues     func(*domain.ElectricityState, *domain.ElectricityUsage)
	reader         *bufio.Reader
}

func NewElectricitySerialMeter(name string, role domain.EnergySourceRole, brand string, speed uint16, url string, lineIndices []uint8, attributes string) domain.ElectricityMeter {
	genenme := &genericEnergyMeter{
		name: name,
	}
	genelme := &genericElectricityMeter{
		role: role,
	}
	esm := &serialElectricityMeter{
		url: url,
	}
	probeBaudRates := []uint{115200, 57600, 38400, 19200, 9600}
	if speed != 0 {
		probeBaudRates = []uint{uint(speed)}
	}

	esm.probeWaitGroup.Add(1)
	go esm.probeMeter(genenme, genelme, probeBaudRates, brand)
	return esm
}

func (sem *serialElectricityMeter) WaitForInitialization() bool {
	sem.probeWaitGroup.Wait()
	return sem.serialPort != nil
}

func (sem *serialElectricityMeter) StartReading(ctx context.Context) {
	if !sem.WaitForInitialization() {
		log.Warningf("Unable to read values from electricity meter %s in role %v as it is an unknown device", sem.name, sem.role)
	}
	state := domain.NewElectricityState()
	usage := domain.NewElectricityUsage()

	go func() {
		for {
			select {
			case <-ctx.Done():
				sem.shutdown()
				return
			default:
				sem.readValues(state, usage)
				event := domain.NewElectricityMeterValues().
					SetName(sem.Name()).
					SetRole(sem.Role()).
					SetMeterPhases(sem.Phases()).
					SetElectricityState(state).
					SetElectricityUsage(usage)
				domain.ElectricityMeterReadings.Trigger(event)
			}
		}
	}()
}

func (sem *serialElectricityMeter) probeMeter(genenme *genericEnergyMeter, genelme *genericElectricityMeter, baudRates []uint, brand string) bool {
	defer sem.probeWaitGroup.Done()
	for _, rate := range baudRates {
		serialPort, err := serial.Open(&serial.Config{
			Address:  sem.url,
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
				log.Infof("Probing for DSMR meter with baud rate %d at %s", rate, sem.url)
			}
			d := &dsmrGridMeter{
				genenme,
				genelme,
				sem,
			}
			if d.probe(serialPort) {
				return true
			}
		}
		_ = serialPort.Close()
	}
	return false
}

func (sem *serialElectricityMeter) shutdown() {
	if sem.serialPort != nil {
		log.Infof("Stop reading values from electricity meter %s in role %v", sem.name, sem.role)
		err := sem.serialPort.Close()
		if err != nil && log.DebugEnabled() {
			log.Debugf("Unable to close serial port: %v", err)
		}
		sem.serialPort = nil
	}
}

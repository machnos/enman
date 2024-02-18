package meters

import (
	"bufio"
	"context"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/serial"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type dsmrMeter struct {
	*energyMeter
	*electricityMeter
	*gasMeter
	*serialMeter
	runContext                     context.Context
	cancelFunc                     context.CancelFunc
	shutdownWaitGroup              sync.WaitGroup
	electricityState               *domain.ElectricityState
	electricityUsage               *domain.ElectricityUsage
	gasUsage                       *domain.GasUsage
	gasMeterReferenceChannelPrefix string
	mbusClientValue                *regexp.Regexp
}

func newDsmrMeter(serialConfig *serial.Config, meterConfig *config.EnergyMeter) (domain.EnergyMeter, error) {
	enMe := newEnergyMeter("DSMR")
	elMe := newElectricityMeter(meterConfig)
	gaMe := newGasMeter()
	seMe := newSerialMeter(serialConfig)
	dsmr := &dsmrMeter{
		energyMeter:      enMe,
		electricityMeter: elMe,
		gasMeter:         gaMe,
		serialMeter:      seMe,
		electricityState: domain.NewElectricityState(),
		electricityUsage: domain.NewElectricityUsage(),
		gasUsage:         domain.NewGasUsage(),
	}
	return dsmr, dsmr.validMeter()
}
func (d *dsmrMeter) validMeter() error {
	serialPort, err := serial.Open(d.serialConfig)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(serialPort)
	message, err := reader.ReadString('\x21')
	if err != nil {
		return err
	}
	lines := strings.Split(message, string('\n'))
	found := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "1-3:0.2.8") {
			dsmrValue := d.float32ValueFromObisLine(line)
			if dsmrValue != 50 {
				_ = serialPort.Close()
				return fmt.Errorf("detected a %s grid meter with an unsupported version %v at %s. Meter will not be queried for values", d.Brand(), dsmrValue, d.serialPort)
			}
			log.Infof("Detected a %s meter at %s.", d.Brand(), d.serialConfig.Address)
			d.serialPort = serialPort
			d.reader = reader
			d.model = "DSMR"
			d.runContext, d.cancelFunc = context.WithCancel(context.Background())
			d.mbusClientValue, _ = regexp.Compile("\\(.*\\)\\((.*)\\*.*\\)")
			d.startUpdateLoop()
			found = true
		} else if strings.HasPrefix(line, "0-0:96.1.0") {
			d.serial = d.stringValueFromObisLine(line)
		} else if strings.HasPrefix(line, "1-0:52.7.0") {
			if d.phases < 2 {
				d.phases = 2
			}
		} else if strings.HasPrefix(line, "1-0:72.7.0") {
			if d.phases < 3 {
				d.phases = 3
			}
		} else if strings.HasPrefix(line, "0-1:24.1.0") {
			mbusDevice := d.stringValueFromObisLine(line)
			switch mbusDevice {
			case "003":
				d.gasMeterReferenceChannelPrefix = "0-1"
			}
		} else if strings.HasPrefix(line, "0-2:24.1.0") {
			mbusDevice := d.stringValueFromObisLine(line)
			switch mbusDevice {
			case "003":
				d.gasMeterReferenceChannelPrefix = "0-2"
			}
		} else if strings.HasPrefix(line, "0-3:24.1.0") {
			mbusDevice := d.stringValueFromObisLine(line)
			switch mbusDevice {
			case "003":
				d.gasMeterReferenceChannelPrefix = "0-3"
			}
		} else if strings.HasPrefix(line, "0-4:24.1.0") {
			mbusDevice := d.stringValueFromObisLine(line)
			switch mbusDevice {
			case "003":
				d.gasMeterReferenceChannelPrefix = "0-4"
			}
		}
	}
	if found {
		// keep serial port open and return without error
		d.setDefaultLineIndices(fmt.Sprintf("%s %s at %s", d.brand, d.model, d.serialConfig.Address))
		return nil
	}
	_ = serialPort.Close()
	return fmt.Errorf("%s grid meter not found at %s", d.Brand(), d.serialConfig.Address)
}

func (d *dsmrMeter) UpdateValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, gasUsage *domain.GasUsage, _ *domain.WaterUsage, _ *domain.BatteryState) {
	if d.HasStateAttribute() {
		electricityState.SetValues(d.electricityState)
	}
	if d.HasUsageAttribute() {
		electricityUsage.SetValues(d.electricityUsage)
	}
	gasUsage.SetValues(d.gasUsage)
	json, _ := d.electricityUsage.MarshalJSON()
	println(string(json))
}

func (d *dsmrMeter) Shutdown() {
	log.Infof("Shutting down DSMR meter at %s.", d.serialConfig.Address)
	d.cancelFunc()
	d.shutdownWaitGroup.Wait()
	d.serialMeter.shutdown()
}

func (d *dsmrMeter) float32ValueFromObisLine(obisLine string) float32 {
	return float32(d.float64ValueFromObisLine(obisLine))
}

func (d *dsmrMeter) float64ValueFromObisLine(obisLine string) float64 {
	// TODO waarde kan soms unparsable zijn, bijv 1-0:31.7.0(kW)
	value := d.stringValueFromObisLine(obisLine)
	float, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return math.Ceil(float*1000) / 1000
}

func (d *dsmrMeter) stringValueFromObisLine(obisLine string) string {
	value := obisLine[strings.Index(obisLine, "(")+1 : strings.Index(obisLine, ")")]
	if strings.Index(value, "*") != -1 {
		value = value[0:strings.Index(value, "*")]
	}
	return value
}

func (d *dsmrMeter) startUpdateLoop() {
	d.shutdownWaitGroup.Add(1)
	go func() {
		defer d.shutdownWaitGroup.Done()
		for {
			select {
			case <-d.runContext.Done():
				return
			default:
				message, err := d.reader.ReadString('\x21')
				if err != nil {
					return
				}
				if d.phases < 1 {
					d.phases = 1
				}
				totalEnergyConsumed := float64(0)
				totalEnergyProvided := float64(0)
				lines := strings.Split(message, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if d.HasUsageAttribute() {
						if strings.HasPrefix(line, "1-0:1.8.1") {
							totalEnergyConsumed += d.float64ValueFromObisLine(line)
						} else if strings.HasPrefix(line, "1-0:1.8.2") {
							totalEnergyConsumed += d.float64ValueFromObisLine(line)
						} else if strings.HasPrefix(line, "1-0:2.8.1") {
							totalEnergyProvided += d.float64ValueFromObisLine(line)
						} else if strings.HasPrefix(line, "1-0:2.8.2") {
							totalEnergyProvided += d.float64ValueFromObisLine(line)
						}
					}
					if d.HasStateAttribute() {
						if strings.HasPrefix(line, "1-0:32.7.0") {
							d.electricityState.SetVoltage(0, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:52.7.0") {
							d.electricityState.SetVoltage(1, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:72.7.0") {
							d.electricityState.SetVoltage(2, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:31.7.0") {
							d.electricityState.SetCurrent(0, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:51.7.0") {
							d.electricityState.SetCurrent(1, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:71.7.0") {
							d.electricityState.SetCurrent(2, d.float32ValueFromObisLine(line))
						} else if strings.HasPrefix(line, "1-0:21.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(0, value*1000)
							}
						} else if strings.HasPrefix(line, "1-0:41.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(1, value*1000)
							}
						} else if strings.HasPrefix(line, "1-0:61.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(2, value*1000)
							}
						} else if strings.HasPrefix(line, "1-0:22.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(0, value*-1000)
							}
						} else if strings.HasPrefix(line, "1-0:42.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(1, value*-1000)
							}
						} else if strings.HasPrefix(line, "1-0:62.7.0") {
							value := d.float32ValueFromObisLine(line)
							if value > 0 {
								d.electricityState.SetPower(2, value*-1000)
							}
						} else if strings.HasPrefix(line, "1-0:52.7.0") {
							if d.phases < 2 {
								d.phases = 2
							}
						} else if strings.HasPrefix(line, "1-0:72.7.0") {
							if d.phases < 3 {
								d.phases = 3
							}
						}
					}
					if strings.HasPrefix(line, d.gasMeterReferenceChannelPrefix+":24.2.1") {
						result := d.mbusClientValue.FindStringSubmatch(line)
						if result != nil && len(result) == 2 {
							float, err := strconv.ParseFloat(result[1], 64)
							if err != nil {
								continue
							}
							d.gasUsage.SetGasConsumed(math.Ceil(float*1000) / 1000)
						}
					}
				}
				if d.HasUsageAttribute() {
					d.electricityUsage.SetTotalEnergyConsumed(totalEnergyConsumed)
					d.electricityUsage.SetTotalEnergyProvided(totalEnergyProvided)
				}
			}
		}
	}()
}

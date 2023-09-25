package meters_old

import (
	"bufio"
	"context"
	"enman/internal/domain"
	"enman/internal/log"
	"enman/internal/serial"
	"strconv"
	"strings"
)

type dsmrGridMeter struct {
	*genericEnergyMeter
	*genericElectricityMeter
	*serialElectricityMeter
}

func (d *dsmrGridMeter) probe(serialPort serial.Port) bool {
	reader := bufio.NewReader(serialPort)
	buf := make([]byte, 2048)
	_, err := reader.Read(buf)
	if err != nil {
		return false
	}
	lines := strings.Split(string(buf), string('\n'))
	for _, line := range lines {
		if strings.HasPrefix(line, "1-3:0.2.8") {
			dsmrValue := d.valueFromObisLine(line)
			if dsmrValue != 50 {
				log.Infof("Detected a DSMR electricity meter with an unsupported version %v at %s. Meter will not be queried for values.", dsmrValue, d.url)
				return false
			}
			log.Infof("Detected a DSMR meter at %s.", d.url)
			d.serialPort = serialPort
			d.serialElectricityMeter.readValues = d.readValues
			d.reader = reader
			return true
		}
	}
	return false
}

func (d *dsmrGridMeter) readValues(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
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
	for ix := 0; ix < len(lines); ix++ {
		trimmedLine := strings.TrimSpace(lines[ix])
		if d.HasUsageAttribute() && electricityUsage != nil {
			if strings.HasPrefix(trimmedLine, "1-0:1.8.1.255") {
				totalEnergyConsumed += float64(d.valueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:1.8.2.255") {
				totalEnergyConsumed += float64(d.valueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.1.255") {
				totalEnergyProvided += float64(d.valueFromObisLine(trimmedLine) * 1000)
			} else if strings.HasPrefix(trimmedLine, "1-0:2.8.2.255") {
				totalEnergyProvided += float64(d.valueFromObisLine(trimmedLine) * 1000)
			}
		}
		if d.HasStateAttribute() && electricityState != nil {
			if strings.HasPrefix(trimmedLine, "1-0:32.7.0") {
				electricityState.SetVoltage(0, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:52.7.0") {
				electricityState.SetVoltage(1, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:72.7.0") {
				electricityState.SetVoltage(2, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:31.7.0") {
				electricityState.SetCurrent(0, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:51.7.0") {
				electricityState.SetCurrent(1, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:71.7.0") {
				electricityState.SetCurrent(2, d.valueFromObisLine(trimmedLine))
			} else if strings.HasPrefix(trimmedLine, "1-0:21.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(0, value*1000)
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:41.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(1, value*1000)
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:61.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(2, value*1000)
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:22.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(0, value*-1000)
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:42.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(1, value*-1000)
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:62.7.0") {
				value := d.valueFromObisLine(trimmedLine)
				if value > 0 {
					electricityState.SetPower(2, value*-1000)
				}
			}
			if strings.HasPrefix(trimmedLine, "1-0:52.7.0") {
				if d.phases < 2 {
					d.phases = 2
				}
			} else if strings.HasPrefix(trimmedLine, "1-0:72.7.0") {
				if d.phases < 3 {
					d.phases = 3
				}
			}
		}
	}
	if d.HasUsageAttribute() && electricityUsage != nil {
		// TODO add KWH per phase, see https://www.netbeheernederland.nl/_upload/Files/Slimme_meter_15_a727fce1f1.pdf
		electricityUsage.SetTotalEnergyConsumed(totalEnergyConsumed)
		electricityUsage.SetTotalEnergyProvided(totalEnergyProvided)
	}
}

func (d *dsmrGridMeter) readValues2(electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage, ctx context.Context) {
	reader := bufio.NewReader(d.serialPort)
	for {
		select {
		case <-ctx.Done():
			_ = d.serialPort.Close()
			return
		default:
			message, err := reader.ReadString('\x21')
			if err != nil {
				continue
			}
			totalEnergyConsumed := float64(0)
			totalEnergyProvided := float64(0)
			nrOfPhases := uint8(1)
			lines := strings.Split(message, "\n")
			for ix := 0; ix < len(lines); ix++ {
				trimmedLine := strings.TrimSpace(lines[ix])
				if strings.HasPrefix(trimmedLine, "1-0:1.8.1.255") {
					totalEnergyConsumed += float64(d.valueFromObisLine(trimmedLine) * 1000)
				} else if strings.HasPrefix(trimmedLine, "1-0:1.8.2.255") {
					totalEnergyConsumed += float64(d.valueFromObisLine(trimmedLine) * 1000)
				} else if strings.HasPrefix(trimmedLine, "1-0:2.8.1.255") {
					totalEnergyProvided += float64(d.valueFromObisLine(trimmedLine) * 1000)
				} else if strings.HasPrefix(trimmedLine, "1-0:2.8.2.255") {
					totalEnergyProvided += float64(d.valueFromObisLine(trimmedLine) * 1000)
				} else if strings.HasPrefix(trimmedLine, "1-0:32.7.0") {
					electricityState.SetVoltage(0, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:52.7.0") {
					if nrOfPhases < 2 {
						nrOfPhases = 2
					}
					electricityState.SetVoltage(1, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:72.7.0") {
					if nrOfPhases < 3 {
						nrOfPhases = 3
					}
					electricityState.SetVoltage(2, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:31.7.0") {
					electricityState.SetCurrent(0, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:51.7.0") {
					electricityState.SetCurrent(1, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:71.7.0") {
					electricityState.SetCurrent(2, d.valueFromObisLine(trimmedLine))
				} else if strings.HasPrefix(trimmedLine, "1-0:21.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(0, value*1000)
					}
				} else if strings.HasPrefix(trimmedLine, "1-0:41.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(1, value*1000)
					}
				} else if strings.HasPrefix(trimmedLine, "1-0:61.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(2, value*1000)
					}
				} else if strings.HasPrefix(trimmedLine, "1-0:22.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(0, value*-1000)
					}
				} else if strings.HasPrefix(trimmedLine, "1-0:42.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(1, value*-1000)
					}
				} else if strings.HasPrefix(trimmedLine, "1-0:62.7.0") {
					value := d.valueFromObisLine(trimmedLine)
					if value > 0 {
						electricityState.SetPower(2, value*-1000)
					}
				}
			}
			// TODO add KWH per phase, see https://www.netbeheernederland.nl/_upload/Files/Slimme_meter_15_a727fce1f1.pdf
			electricityUsage.SetTotalEnergyConsumed(totalEnergyConsumed)
			electricityUsage.SetTotalEnergyProvided(totalEnergyProvided)

			event := domain.NewElectricityMeterValues().
				SetName(d.name).
				SetRole(d.role).
				SetMeterPhases(nrOfPhases).
				SetElectricityState(electricityState).
				SetElectricityUsage(electricityUsage)
			domain.ElectricityMeterReadings.Trigger(event)
		}
	}
}

func (d *dsmrGridMeter) valueFromObisLine(obisLine string) float32 {
	// TODO waarde kan soms unparsable zijn, bijv 1-0:31.7.0(kW)
	value := obisLine[strings.Index(obisLine, "(")+1 : strings.Index(obisLine, ")")]
	if strings.Index(value, "*") != -1 {
		value = value[0:strings.Index(value, "*")]
	}
	float, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0
	}
	return float32(float)
}

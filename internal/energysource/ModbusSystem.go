package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"runtime"
	"time"
)

type modbusGrid struct {
	*energysource.GridBase
	modbusUnitId uint8
	meterCode    string
	meterType    string
}

type modbusPv struct {
	*energysource.PvBase
	modbusUnitId uint8
	meterCode    string
	meterType    string
}

type ModbusConfig struct {
	modbusUrl        string
	modbusSpeed      uint16
	timeout          time.Duration
	gridConfig       *energysource.GridConfig
	modbusGridConfig *ModbusGridConfig
	pvConfigs        []*ModbusPvConfig
	updateGridValues func(*modbus.ModbusClient, *modbusGrid)
	updatePvValues   func(*modbus.ModbusClient, *modbusPv)
}

type ModbusGridConfig struct {
	modbusUnitId uint8
	initialize   func(*modbus.ModbusClient, *modbusGrid) error
}

type ModbusPvConfig struct {
	modbusUnitId uint8
	initialize   func(*modbus.ModbusClient, *modbusPv) error
}

func NewModbusSystem(config *ModbusConfig) (*energysource.System, error) {
	modbusConfig := &modbus.ClientConfiguration{
		URL:     config.modbusUrl,
		Timeout: config.timeout,
	}
	if config.modbusSpeed > 0 {
		modbusConfig.Speed = uint(config.modbusSpeed)
	}
	modbusClient, err := modbus.NewClient(modbusConfig)

	if err != nil {
		return nil, err
	}
	err = modbusClient.Open()
	if err != nil {
		return nil, err
	}
	var grid *energysource.Grid = nil
	if config.modbusGridConfig != nil {
		mbg, err := newModbusGrid(modbusClient, config.gridConfig, config.modbusGridConfig)
		if err != nil {
			return nil, err
		}
		e := energysource.Grid(mbg)
		grid = &e
	}
	var pvs []*energysource.Pv = nil
	for ix := 0; ix < len(config.pvConfigs); ix++ {
		mbpv, err := newModbusPv(modbusClient, &energysource.PvConfig{}, config.pvConfigs[ix])
		if err != nil {
			return nil, err
		}
		pv := energysource.Pv(mbpv)
		pvs = append(pvs, &pv)
	}
	var system = energysource.NewSystem(grid, pvs)
	go readSystemValues(modbusClient, system, config)
	return system, nil
}

func readSystemValues(client *modbus.ModbusClient, system *energysource.System, config *ModbusConfig) {
	ticker := time.NewTicker(time.Millisecond * 250)
	tickerChannel := make(chan bool)
	runtime.SetFinalizer(system, func(a *energysource.System) {
		tickerChannel <- true
		ticker.Stop()
	})
	defer func(client *modbus.ModbusClient) {
		_ = client.Close()
	}(client)

	for {
		select {
		case <-ticker.C:
			if system.Grid() != nil {
				modbusGrid, ok := (*system.Grid()).(*modbusGrid)
				if ok {
					config.updateGridValues(client, modbusGrid)
				}
			}
			if system.Pvs() != nil {
				for ix := 0; ix < len(system.Pvs()); ix++ {
					modbusPv, ok := (*system.Pvs()[ix]).(*modbusPv)
					if ok {
						config.updatePvValues(client, modbusPv)
					}
				}
			}
		case <-tickerChannel:
			return
		}
	}
}

func newModbusGrid(modbusClient *modbus.ModbusClient, gridConfig *energysource.GridConfig, config *ModbusGridConfig) (*modbusGrid, error) {
	mg := &modbusGrid{
		GridBase:     energysource.NewGrid(gridConfig),
		modbusUnitId: config.modbusUnitId,
	}
	if config.initialize != nil {
		err := config.initialize(modbusClient, mg)
		if err != nil {
			return nil, err
		}
	}
	return mg, nil
}

func newModbusPv(modbusClient *modbus.ModbusClient, pvConfig *energysource.PvConfig, config *ModbusPvConfig) (*modbusPv, error) {
	mpv := &modbusPv{
		PvBase:       energysource.NewPv(pvConfig),
		modbusUnitId: config.modbusUnitId,
	}
	if config.initialize != nil {
		err := config.initialize(modbusClient, mpv)
		if err != nil {
			return nil, err
		}
	}
	return mpv, nil
}

func getValueFromRegisterResultArray(values []uint16, ix uint8, scaleFactor float32, defaultValue float32) float32 {
	if values == nil {
		return defaultValue
	}
	value := float32(int16(values[ix]))
	if scaleFactor != 0 {
		value /= scaleFactor
	}
	return value
}

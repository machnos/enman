package energysource

import (
	"enman/internal/modbus"
	"enman/pkg/energysource"
	"time"
)

type ModbusConfig struct {
	ModbusUrl string
	Timeout   time.Duration
	Speed     uint
}

type ModbusGridConfig struct {
	Grid         *energysource.GridConfig
	ModbusMeters []*ModbusMeterConfig
}

type ModbusPvConfig struct {
	Pv           *energysource.PvConfig
	ModbusMeters []*ModbusMeterConfig
}

type ModbusMeterConfig struct {
	ModbusUnitId uint8
	LineIndexes  []uint8
}

type modbusGrid struct {
	*energysource.GridBase
	meters []*modbusMeter
}

type modbusPv struct {
	*energysource.PvBase
	meters []*modbusMeter
}

type modbusMeter interface {
	initialize(client *modbus.ModbusClient, config *ModbusMeterConfig) error
}

func NewModbusSystem(config *ModbusConfig,
	gridConfig *ModbusGridConfig,
	pvConfigs []*ModbusPvConfig,
	newMeter func() modbusMeter,
	updateSystemValues func(client *modbus.ModbusClient, system *energysource.System),
) (*energysource.System, error) {
	modbusConfig := &modbus.ClientConfiguration{
		URL:     config.ModbusUrl,
		Timeout: time.Millisecond * 500,
		Speed:   9600,
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
	if gridConfig != nil && gridConfig.ModbusMeters != nil {
		var gridMeters []*modbusMeter = nil
		for _, modbusMeter := range gridConfig.ModbusMeters {
			meter := newMeter()
			err := meter.initialize(modbusClient, modbusMeter)
			if err != nil {
				return nil, err
			}
			gridMeters = append(gridMeters, &meter)
		}
		g := energysource.Grid(modbusGrid{
			GridBase: energysource.NewGridBase(gridConfig.Grid),
			meters:   gridMeters,
		})
		grid = &g
	}
	var pvs []*energysource.Pv = nil
	if pvConfigs != nil {
		for _, modbusPvConfig := range pvConfigs {
			var pvMeters []*modbusMeter = nil
			for _, modbusMeter := range modbusPvConfig.ModbusMeters {
				meter := newMeter()
				err := meter.initialize(modbusClient, modbusMeter)
				if err != nil {
					return nil, err
				}
				pvMeters = append(pvMeters, &meter)
			}
			pv := energysource.Pv(modbusPv{
				PvBase: energysource.NewPvBase(modbusPvConfig.Pv),
				meters: pvMeters,
			})
			pvs = append(pvs, &pv)
		}
	}
	system := energysource.NewSystem(grid, pvs)
	go updateSystemValues(modbusClient, system)
	return system, nil
}

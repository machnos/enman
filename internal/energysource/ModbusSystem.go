package energysource

import (
	"enman/pkg/energysource"
)

type ModbusGridConfig struct {
	GridConfig  *energysource.GridConfig
	ModbusMeter *ModbusMeter
}

type ModbusPvConfig struct {
	PvConfig    *energysource.PvConfig
	ModbusMeter *ModbusMeter
}

type ModbusMeter struct {
	ModbusUnitId uint8
	LineIndexes  []uint8
}

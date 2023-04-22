package modbus

import (
	"enman/internal"
	"enman/internal/energysource"
	"enman/internal/log"
	"enman/internal/modbus"
	"time"
)

type Config struct {
	ModbusUrl string
	Timeout   time.Duration
	Speed     uint
}

type GridConfig struct {
	*energysource.GridConfig
	ModbusMeters []*MeterConfig
}

type PvConfig struct {
	*energysource.PvConfig
	ModbusMeters []*MeterConfig
}

type MeterConfig struct {
	ModbusUnitId uint8
	LineIndices  []uint8
}

type Grid struct {
	*energysource.GridBase
	Meters []*Meter
}

type Pv struct {
	*energysource.PvBase
	Meters []*Meter
}

type Meter interface {
	SerialNumber() string
	Initialize(client *modbus.ModbusClient, config *MeterConfig) error
}

type UpdateLoop struct {
	Client             *modbus.ModbusClient
	UpdateChannels     *internal.UpdateChannels
	grid               *Grid
	gridUpdateFunction func(*modbus.ModbusClient, *Grid, bool, chan energysource.Grid)
	pvs                map[*Pv]func(*modbus.ModbusClient, *Pv, bool, chan energysource.Pv)
	stopChannel        chan bool
}

func NewUpdateLoop(config *modbus.ClientConfiguration, updateChannels *internal.UpdateChannels) (*UpdateLoop, error) {
	client, err := newModbusClient(config)
	if err != nil {
		return nil, err
	}
	u := &UpdateLoop{
		Client:         client,
		UpdateChannels: updateChannels,
		pvs:            make(map[*Pv]func(*modbus.ModbusClient, *Pv, bool, chan energysource.Pv)),
	}
	go u.startUpdate()
	return u, nil
}

func (u *UpdateLoop) startUpdate() {
	pollInterval := uint16(250)
	log.Infof("Start polling modbus devices at %s every %d milliseconds", u.Client.URL(), pollInterval)
	ticker := time.NewTicker(time.Millisecond * time.Duration(pollInterval))
	u.stopChannel = make(chan bool)
	var runMinute = -1
	for {
		select {
		case <-ticker.C:
			updateKwhTotals := false
			_, minutes, _ := time.Now().Clock()
			if runMinute != minutes {
				runMinute = minutes
				updateKwhTotals = true
			}
			if u.grid != nil {
				u.gridUpdateFunction(u.Client, u.grid, updateKwhTotals, u.UpdateChannels.GridUpdated())
			}
			for pv, updateFunction := range u.pvs {
				updateFunction(u.Client, pv, updateKwhTotals, u.UpdateChannels.PvUpdated())
			}
		case <-u.stopChannel:
			ticker.Stop()
			return
		}
	}
}

func (u *UpdateLoop) Close() {
	err := u.Client.Close()
	if err != nil {
		log.Warningf("Failed to close modbus connection: %s", err.Error())
	}
	if u.stopChannel != nil {
		u.stopChannel <- true
	}
}

func (u *UpdateLoop) registerGridUpdateFunction(grid *Grid, updateFunction func(*modbus.ModbusClient, *Grid, bool, chan energysource.Grid)) {
	u.grid = grid
	u.gridUpdateFunction = updateFunction
}

func (u *UpdateLoop) registerPvUpdateFunction(pv *Pv, updateFunction func(*modbus.ModbusClient, *Pv, bool, chan energysource.Pv)) {
	u.pvs[pv] = updateFunction
}

func newModbusClient(config *modbus.ClientConfiguration) (*modbus.ModbusClient, error) {
	modbusClient, err := modbus.NewClient(config)
	if err != nil {
		return nil, err
	}
	err = modbusClient.Open()
	if err != nil {
		return nil, err
	}
	return modbusClient, nil
}

func NewGrid(name string,
	config *GridConfig,
	newMeter func() Meter,
	updateLoop *UpdateLoop,
	update func(*modbus.ModbusClient, *Grid, bool, chan energysource.Grid),
) (*Grid, error) {
	var meters []*Meter = nil
	for _, mbMeter := range config.ModbusMeters {
		meter := newMeter()
		err := meter.Initialize(updateLoop.Client, mbMeter)
		if err != nil {
			return nil, err
		}
		meters = append(meters, &meter)
	}
	g := &Grid{
		GridBase: energysource.NewGridBase(name, config.GridConfig),
		Meters:   meters,
	}
	updateLoop.registerGridUpdateFunction(g, update)
	return g, nil
}

func NewPv(name string,
	config *PvConfig,
	newMeter func() Meter,
	updateLoop *UpdateLoop,
	update func(*modbus.ModbusClient, *Pv, bool, chan energysource.Pv),
) (*Pv, error) {
	var meters []*Meter = nil
	for _, mbMeter := range config.ModbusMeters {
		meter := newMeter()
		err := meter.Initialize(updateLoop.Client, mbMeter)
		if err != nil {
			return nil, err
		}
		meters = append(meters, &meter)
	}
	pv := &Pv{
		PvBase: energysource.NewPvBase(name, config.PvConfig),
		Meters: meters,
	}
	updateLoop.registerPvUpdateFunction(pv, update)
	return pv, nil
}

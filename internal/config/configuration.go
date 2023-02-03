package config

import (
	"encoding/json"
	"enman/internal/log"
	"os"
)

type Brand string

const (
	CarloGavazzi Brand = "Carlo Gavazzi"
	Victron      Brand = "Victron"
	DSMR         Brand = "DSMR"
)

type Configuration struct {
	Http   *Http   `json:"http"`
	Grid   *Grid   `json:"grid"`
	Pvs    []*Pv   `json:"pvs"`
	Influx *Influx `json:"influx"`
}

type Grid struct {
	Name       string         `json:"name"`
	ConnectURL string         `json:"connect_url"`
	Brand      Brand          `json:"brand"`
	Voltage    uint16         `json:"voltage"`
	MaxCurrent float32        `json:"max_current"`
	Phases     uint8          `json:"phases"`
	Meters     []*ModbusMeter `json:"meters"`
}

type Pv struct {
	Name       string         `json:"name"`
	ConnectURL string         `json:"connect_url"`
	Brand      Brand          `json:"brand"`
	Meters     []*ModbusMeter `json:"meters"`
}

type ModbusMeter struct {
	ModbusUnitId uint8   `json:"modbus_unit_id"`
	LineIndices  []uint8 `json:"line_indices"`
}

type Influx struct {
	ServerUrl string `json:"server_url"`
	Token     string `json:"token"`
}

type Http struct {
	Port uint16 `json:"port"`
}

func LoadConfiguration(configFile string) *Configuration {
	file, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("Unable to load configuration file: %s", err.Error())
		panic(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	decoder := json.NewDecoder(file)
	configuration := &Configuration{}
	err = decoder.Decode(configuration)
	if err != nil {
		log.Fatalf("Unable to load configuration file: %s", err.Error())
		panic(err)
	}
	return configuration
}

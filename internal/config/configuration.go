package config

import (
	"encoding/json"
	"enman/internal/log"
	"os"
	"strings"
	"time"
)

type Brand string

const (
	ABB          Brand = "ABB"
	CarloGavazzi Brand = "Carlo Gavazzi"
	Victron      Brand = "Victron"
	DSMR         Brand = "DSMR"
)

type Configuration struct {
	Http          *Http            `json:"http"`
	Grid          *Grid            `json:"grid"`
	Pvs           []*Pv            `json:"pvs"`
	Persistency   *Persistency     `json:"persistency"`
	ModbusServers []*ModbusServers `json:"modbus_servers"`
	Prices        *Prices          `json:"prices"`
}

type Grid struct {
	Name                 string                `json:"name"`
	ConnectURL           string                `json:"connect_url"`
	Brand                Brand                 `json:"brand"`
	Voltage              uint16                `json:"voltage"`
	MaxCurrent           float32               `json:"max_current"`
	Phases               uint8                 `json:"phases"`
	Meters               []*ModbusMeter        `json:"meters"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator"`
}

type Pv struct {
	Name                 string                `json:"name"`
	ConnectURL           string                `json:"connect_url"`
	Brand                Brand                 `json:"brand"`
	Meters               []*ModbusMeter        `json:"meters"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator"`
}

type ModbusMeter struct {
	ModbusUnitId uint8   `json:"modbus_unit_id"`
	LineIndices  []uint8 `json:"line_indices"`
}

type ModbusMeterSimulator struct {
	ModbusUnitId uint8  `json:"modbus_unit_id"`
	MeterType    string `json:"meter_type"`
}

type Persistency struct {
	Influx *Influx `json:"influx"`
}

type Influx struct {
	ServerUrl string `json:"server_url"`
	Token     string `json:"token"`
}

type ModbusServers struct {
	ServerUrl  string `json:"server_url"`
	Speed      uint16 `json:"speed"`
	DataBits   uint8  `json:"data_bits"`
	Parity     uint8  `json:"parity"`
	StopBits   uint8  `json:"stop_bits"`
	Timeout    uint16 `json:"timeout"`
	MaxClients uint8  `json:"max_clients"`
}

type Prices struct {
	Country   string           `json:"country"`
	Area      string           `json:"area"`
	Providers []EnergyProvider `json:"providers"`
	Entsoe    Entsoe           `json:"entsoe"`
}

type Entsoe struct {
	SecurityToken string `json:"security_token"`
}

type EnergyProvider struct {
	Name        string       `json:"name"`
	PriceModels []PriceModel `json:"price_models"`
}

type PriceModel struct {
	Start                YearMonthDay `json:"start"`
	Vat                  float32      `json:"vat"`
	AdditionalCostPerKwh float32      `json:"additional_cost_per_kwh"`
}
type YearMonthDay time.Time

func (j *YearMonthDay) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*j = YearMonthDay(t)
	return nil
}

func (j *YearMonthDay) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(*j))
}

func (j *YearMonthDay) Format(s string) string {
	t := time.Time(*j)
	return t.Format(s)
}

func (j *YearMonthDay) Time() time.Time {
	return time.Time(*j)
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

package config

import (
	"encoding/json"
	gpv "github.com/go-playground/validator/v10"
	"os"
	"time"
)

type Configuration struct {
	LogLevel      uint8            `json:"log_level"`
	Http          *Http            `json:"http"`
	Grid          *Grid            `json:"grid"`
	Pvs           []*Pv            `json:"pvs" validate:"dive"`
	Persistency   *Persistency     `json:"persistency"`
	ModbusServers []*ModbusServers `json:"modbus_servers" validate:"dive"`
	Prices        *Prices          `json:"prices"`
}

type Grid struct {
	Name                 string                `json:"name" validate:"required"`
	Voltage              uint16                `json:"voltage"`
	MaxCurrent           float32               `json:"max_current" validate:"gte=0"`
	Phases               uint8                 `json:"phases"`
	Meters               []*EnergyMeter        `json:"meters" validate:"dive"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator"`
}

type Pv struct {
	Name                 string                `json:"name" validate:"required"`
	Meters               []*EnergyMeter        `json:"meters" validate:"dive"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator"`
}

type EnergyMeter struct {
	ConnectURL   string   `json:"connect_url"`
	Type         string   `json:"type" validate:"required,oneof=modbus serial"`
	Brand        string   `json:"brand" validate:"oneof='ABB' 'Carlo Gavazzi' 'DSMR' 'Victron' ''"`
	ModbusUnitId uint8    `json:"modbus_unit_id" validate:"required_if=Type modbus"`
	Speed        uint32   `json:"speed"`
	LineIndices  []uint8  `json:"line_indices" validate:"required,gte=1,lte=3"`
	Attributes   []string `json:"attributes" validate:"dive,oneof='state' 'usage' ''"`
}

type ModbusMeterSimulator struct {
	ModbusUnitId uint8  `json:"modbus_unit_id"`
	MeterType    string `json:"meter_type" validate:"oneof=EM24"`
}

type Persistency struct {
	Influx *Influx `json:"influx"`
}

type Influx struct {
	ServerUrl string `json:"server_url" validate:"url"`
	Token     string `json:"token" validate:"required"`
}

type ModbusServers struct {
	ServerUrl  string `json:"server_url" validate:"required,url"`
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
	Providers []EnergyProvider `json:"providers" validate:"dive"`
	Entsoe    Entsoe           `json:"entso-e"`
}

type Entsoe struct {
	SecurityToken string `json:"security_token" validate:"required"`
}

type EnergyProvider struct {
	Name        string       `json:"name" validate:"required"`
	PriceModels []PriceModel `json:"price_models" validate:"dive"`
}

type PriceModel struct {
	Start              string `json:"start" validate:"required,datetime=2006-01-02"`
	ConsumptionFormula string `json:"consumption_formula"`
	FeedbackFormula    string `json:"feedback_formula"`
}

func (p PriceModel) StartAsTime() time.Time {
	date, _ := time.Parse("2006-01-02", p.Start)
	return date
}

type Http struct {
	Port        uint16 `json:"port"`
	ContextRoot string `json:"context_root"`
}

func LoadConfiguration(configFile string) (*Configuration, error) {
	file, err := os.Open(configFile)
	if err != nil {
		defer func(file *os.File) {
			_ = file.Close()
		}(file)
		return nil, err
	}

	decoder := json.NewDecoder(file)
	configuration := &Configuration{}
	err = decoder.Decode(configuration)
	if err != nil {
		return nil, err
	}
	err = gpv.New().Struct(configuration)
	if err != nil {
		return nil, err
	}
	return configuration, nil
}

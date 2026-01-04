package config

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	gpv "github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Log           *Log            `json:"log" yaml:"log"`
	Http          *Http           `json:"http" yaml:"http"`
	Grid          *Grid           `json:"grid" yaml:"grid"`
	Pvs           *Pvs            `json:"pvs" yaml:"pvs"`
	AcLoads       []*AcLoad       `json:"ac_loads" yaml:"ac_loads" validate:"dive"`
	Batteries     Batteries       `json:"batteries" yaml:"batteries"`
	Persistency   *Persistency    `json:"persistency" yaml:"persistency"`
	ModbusServers []*ModbusServer `json:"modbus_servers" yaml:"modbus_servers" validate:"dive"`
	Prices        *Prices         `json:"prices" yaml:"prices"`
}

type Log struct {
	Level    uint8         `json:"level" yaml:"level"`
	Packages []*LogPackage `json:"packages" yaml:"packages" validate:"dive"`
}

type LogPackage struct {
	Name  string `json:"name" yaml:"name" validate:"required"`
	Level uint8  `json:"level" yaml:"level"`
}

type Grid struct {
	Name                         string                `json:"name" yaml:"name" json:"required" validate:"required,max=50"`
	Voltage                      uint16                `json:"voltage" yaml:"voltage"`
	MaxCurrent                   float32               `json:"max_current" yaml:"max_current" validate:"gte=0"`
	Phases                       uint8                 `json:"phases" yaml:"phases"`
	ElectricityTargetConsumption int                   `json:"electricity_target_consumption" yaml:"electricity_target_consumption"`
	Meters                       []*EnergyMeter        `json:"meters" yaml:"meters" validate:"dive" `
	ModbusMeterSimulator         *ModbusMeterSimulator `json:"modbus_meter_simulator" yaml:"modbus_meter_simulator"`
	Controller                   *GridController       `json:"controller" yaml:"controller"`
}

type Pvs struct {
	PvStateController *PvStateController `json:"state_controller" yaml:"state_controller"`
	Arrays            []*Pv              `json:"arrays" yaml:"arrays" validate:"dive"`
}

type Pv struct {
	Name                 string                `json:"name" yaml:"name" validate:"required,max=50"`
	Meters               []*EnergyMeter        `json:"meters" yaml:"meters" validate:"dive"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator" yaml:"modbus_meter_simulator"`
	Controller           *PvController         `json:"controller" yaml:"controller"`
}

type AcLoad struct {
	Name                 string                `json:"name" yaml:"name" validate:"required,max=50"`
	Role                 string                `json:"role" yaml:"role" validate:"required,oneof=EvCharger"`
	PercentageFromGrid   uint8                 `json:"percentage_from_grid" yaml:"percentage_from_grid" validate:"gte=0,lte=100"`
	Meters               []*EnergyMeter        `json:"meters" yaml:"meters" validate:"dive"`
	ModbusMeterSimulator *ModbusMeterSimulator `json:"modbus_meter_simulator" yaml:"modbus_meter_simulator"`
}

type Batteries struct {
	Batteries                                     []*Battery `json:"banks" yaml:"banks" validate:"dive"`
	PeakPriceDetectionStandardDeviationMultiplier float32    `json:"peak_price_detection_standard_deviation_multiplier" yaml:"peak_price_detection_standard_deviation_multiplier" validate:"gte=0"`
	SurvivalChargingSocThreshold                  float32    `json:"survival_charging_soc_threshold" yaml:"survival_charging_soc_threshold" validate:"gte=0,lte=100"`
}

type Battery struct {
	Name                   string         `json:"name" yaml:"name" validate:"required,max=50"`
	Capacity               uint16         `json:"capacity" yaml:"capacity" validate:"gt=0"`
	ChargingCoefficient    float32        `json:"charging_coefficient" yaml:"charging_coefficient" validate:"gt=0"`
	DischargingCoefficient float32        `json:"discharging_coefficient" yaml:"discharging_coefficient" validate:"gt=0"`
	Voltage                float32        `json:"voltage" yaml:"voltage" validate:"gt=0"`
	RoundTripEfficiency    float32        `json:"round_trip_efficiency" yaml:"round_trip_efficiency" validate:"gte=0,lte=100"`
	Meters                 []*EnergyMeter `json:"meters" yaml:"meters" validate:"dive"`
}

type EnergyMeter struct {
	ConnectURL         string   `json:"connect_url" yaml:"connect_url"`
	Type               string   `json:"type" yaml:"type" validate:"required,oneof=modbus serial"`
	Brand              string   `json:"brand" yaml:"brand" validate:"oneof='ABB' 'Carlo Gavazzi' 'DSMR' 'Victron' ''"`
	ModbusUnitId       uint8    `json:"modbus_unit_id" yaml:"modbus_unit_id" validate:"required_if=Type modbus"`
	Speed              uint32   `json:"speed" yaml:"speed"`
	LineIndices        []uint8  `json:"line_indices" yaml:"line_indices" validate:"gte=0,lte=3,dive,gte=0,lte=2"`
	Attributes         []string `json:"attributes" yaml:"attributes" validate:"dive,oneof='state' 'current' 'total_current' 'power' 'total_power' 'voltage' 'usage' 'consumption' 'total_consumption' 'production' 'total_production' ''"`
	FailStartupOnError bool     `json:"fail_startup_on_error" yaml:"fail_startup_on_error"`
}

type GridController struct {
	ConnectURL string `json:"connect_url" yaml:"connect_url"`
	Type       string `json:"type" yaml:"type" validate:"required,oneof=modbus"`
	Brand      string `json:"brand" yaml:"brand" validate:"oneof='Victron' ''"`
	Speed      uint32 `json:"speed" yaml:"speed"`
}

type PvController struct {
	ConnectURL string `json:"connect_url" yaml:"connect_url"`
	Type       string `json:"type" yaml:"type" validate:"required,oneof=http"`
	Brand      string `json:"brand" yaml:"brand" validate:"oneof='Shelly' ''"`
}

type PvStateController struct {
	DisableFormula           string `json:"disable_formula" yaml:"disable_formula"`
	BatteryCutoffPercentage  uint8  `json:"battery_cutoff_percentage" yaml:"battery_cutoff_percentage" validate:"gte=0,lte=100,gtfield=BatteryRestartPercentage"`
	BatteryRestartPercentage uint8  `json:"battery_restart_percentage" yaml:"battery_restart_percentage" validate:"gte=0,lte=100,ltfield=BatteryCutoffPercentage"`
}

type ModbusMeterSimulator struct {
	ModbusUnitId uint8  `json:"modbus_unit_id" yaml:"modbus_unit_id" validate:"gte=100"`
	MeterType    string `json:"meter_type" yaml:"meter_type" validate:"oneof=EM24"`
}

type Persistency struct {
	Timescale *Timescale `json:"timescale" yaml:"timescale"`
}

type Timescale struct {
	ConnectionString string `json:"connection_string" yaml:"connection_string" validate:"url"`
}

type ModbusServer struct {
	ServerUrl  string `json:"server_url" yaml:"server_url" validate:"required,url"`
	Speed      uint16 `json:"speed" yaml:"speed"`
	DataBits   uint8  `json:"data_bits" yaml:"data_bits"`
	Parity     uint8  `json:"parity" yaml:"parity"`
	StopBits   uint8  `json:"stop_bits" yaml:"stop_bits"`
	Timeout    uint16 `json:"timeout" yaml:"timeout"`
	MaxClients uint8  `json:"max_clients" yaml:"max_clients"`
}

type Prices struct {
	Country    string            `json:"country" yaml:"country"`
	Area       string            `json:"area" yaml:"area"`
	Providers  []*EnergyProvider `json:"providers" yaml:"providers" validate:"dive"`
	Entsoe     *Entsoe           `json:"entso-e" yaml:"entso-e"`
	EnergyZero bool              `json:"energy_zero" yaml:"energy_zero"`
}

type Entsoe struct {
	SecurityToken string `json:"security_token" yaml:"security_token" validate:"required"`
}

type EnergyProvider struct {
	Name        string        `json:"name" yaml:"name" validate:"required"`
	PriceModels []*PriceModel `json:"price_models" yaml:"price_models" validate:"dive"`
}

type PriceModel struct {
	Start              string `json:"start" yaml:"start" validate:"required,datetime=2006-01-02"`
	EnergyType         string `json:"energy_type" yaml:"energy_type" validate:"required,oneof='electricity' 'gas' 'water'"`
	ConsumptionFormula string `json:"consumption_formula" yaml:"consumption_formula"`
	FeedbackFormula    string `json:"feedback_formula" yaml:"feedback_formula"`
}

func (p PriceModel) StartAsTime() time.Time {
	date, _ := time.Parse("2006-01-02", p.Start)
	return date
}

type Http struct {
	Port          uint16 `json:"port" yaml:"port"`
	ContextRoot   string `json:"context_root" yaml:"context_root"`
	SessionSecret string `json:"session_secret" yaml:"session_secret"`
}

func LoadConfiguration(configFile string) (*Configuration, error) {
	file, err := os.Open(configFile)
	if err != nil {
		defer func(file *os.File) {
			_ = file.Close()
		}(file)
		return nil, err
	}
	configuration := &Configuration{}
	if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(configuration)
	} else {
		decoder := json.NewDecoder(file)
		err = decoder.Decode(configuration)
	}
	if err != nil {
		return nil, err
	}
	validator := gpv.New()
	err = validator.Struct(configuration)
	if err != nil {
		return nil, err
	}
	return configuration, nil
}

func requiredIfParent(fieldValue gpv.FieldLevel) bool {
	parent := fieldValue.Parent()
	parentType := parent.Type().Name()
	if parentType == "Pv" {
		if fieldValue.Field().String() == "" {
			return false
		}
	}
	return true

}

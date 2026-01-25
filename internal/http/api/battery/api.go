package battery

import (
	"enman/internal/domain"
	"enman/internal/domain/repository"
	"enman/internal/http/api"
	"enman/internal/log"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

const (
	errorCodeBatteryRoot            = "-battery"
	errorCodeStartDateParseError    = errorCodeBatteryRoot + "-01"
	errorCodeEndDateParseError      = errorCodeBatteryRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeBatteryRoot + "-03"
	errorCodeUnableToLoadStates     = errorCodeBatteryRoot + "-04"
	errorCodeUnableToLoadSchedule   = errorCodeBatteryRoot + "-05"
	errorCodeUnableToLoadSources    = errorCodeBatteryRoot + "-07"
)

type Api struct {
	*api.BaseApi
	repository repository.Battery
}

func NewApi(system *domain.System, repository repository.Battery) *Api {
	return &Api{
		api.NewBaseApi(system),
		repository,
	}
}

func (api *Api) sources(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Sources []string `json:"sources"`
	}{}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	sources, err := api.repository.BatterySourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (api *Api) states(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		StartTime time.Time      `json:"start_time"`
		EndTime   time.Time      `json:"end_time"`
		States    map[string]any `json:"states"`
	}
	type source struct {
		Role    string    `json:"role"`
		Buckets []*bucket `json:"buckets"`
	}
	type batteryStatesResponse struct {
		Sources map[string]*source `json:"sources"`
	}
	rsp := batteryStatesResponse{
		Sources: make(map[string]*source),
	}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitMinute,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
		CreateEmpty:  false,
	}
	statesRecords, err := api.repository.BatteryStates(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		api.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)

	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadStates, err.Error())
		return
	}
	requestedFields := api.ParseFieldsFromRequestURL(r)
	for _, statesRecord := range statesRecords {
		if rsp.Sources[statesRecord.Name] == nil {
			rsp.Sources[statesRecord.Name] = &source{Role: statesRecord.Role}
		}
		b := &bucket{
			StartTime: statesRecord.StartTime,
			EndTime:   statesRecord.EndTime,
			States:    make(map[string]any),
		}
		for _, fn := range aggregate.Functions {
			stateMap := make(map[string]any)
			api.ConditionallyAddField(requestedFields, "current", statesRecord.States[fn].Current(), stateMap)
			api.ConditionallyAddField(requestedFields, "voltage", statesRecord.States[fn].Voltage(), stateMap)
			api.ConditionallyAddField(requestedFields, "power", statesRecord.States[fn].Power(), stateMap)
			api.ConditionallyAddField(requestedFields, "soc", statesRecord.States[fn].SoC(), stateMap)
			api.ConditionallyAddField(requestedFields, "soh", statesRecord.States[fn].SoH(), stateMap)
			b.States[fn.String()] = stateMap
		}
		rsp.Sources[statesRecord.Name].Buckets = append(rsp.Sources[statesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
}

func (api *Api) scheduleSlots(w http.ResponseWriter, r *http.Request) {
	type scheduleSlot struct {
		StartTime      time.Time `json:"start_time"`
		EndTime        time.Time `json:"end_time"`
		ChargePower    float32   `json:"charge_power"`  // Positive = charging, Negative = discharging
		PredictedSoC   float32   `json:"predicted_soc"` // Predicted state of charge at end of slot
		PricePerKwh    float32   `json:"price_per_kwh"`
		ChargingSource string    `json:"charging_source"` // "grid", "pv", or "none"
		IsCharging     bool      `json:"is_charging"`
		IsDischarging  bool      `json:"is_discharging"`
		IsIdle         bool      `json:"is_idle"`
	}
	type scheduleSlotsResponse struct {
		Slots []*scheduleSlot `json:"slots"`
	}
	rsp := scheduleSlotsResponse{
		Slots: make([]*scheduleSlot, 0),
	}

	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}

	slots, err := api.repository.ScheduleSlots(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSchedule, err.Error())
		return
	}

	for _, slot := range slots {
		rsp.Slots = append(rsp.Slots, &scheduleSlot{
			StartTime:      slot.StartTime,
			EndTime:        slot.EndTime,
			ChargePower:    slot.ChargePower,
			PredictedSoC:   slot.PredictedSoC,
			PricePerKwh:    slot.PricePerKwh,
			ChargingSource: slot.ChargingSource,
			IsCharging:     slot.ChargePower > 0,
			IsDischarging:  slot.ChargePower < 0,
			IsIdle:         slot.ChargePower == 0,
		})
	}
	render.JSON(w, r, rsp)
}

func (api *Api) currentScheduleSlot(w http.ResponseWriter, r *http.Request) {
	type scheduleSlotResponse struct {
		StartTime      time.Time `json:"start_time"`
		EndTime        time.Time `json:"end_time"`
		ChargePower    float32   `json:"charge_power"`
		PredictedSoC   float32   `json:"predicted_soc"`
		PricePerKwh    float32   `json:"price_per_kwh"`
		ChargingSource string    `json:"charging_source"`
		IsCharging     bool      `json:"is_charging"`
		IsDischarging  bool      `json:"is_discharging"`
		IsIdle         bool      `json:"is_idle"`
		Found          bool      `json:"found"`
	}

	slot, err := api.repository.ScheduleSlotAt(time.Now())
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSchedule, err.Error())
		return
	}

	if slot == nil {
		render.JSON(w, r, scheduleSlotResponse{Found: false})
		return
	}

	render.JSON(w, r, scheduleSlotResponse{
		StartTime:      slot.StartTime,
		EndTime:        slot.EndTime,
		ChargePower:    slot.ChargePower,
		PredictedSoC:   slot.PredictedSoC,
		PricePerKwh:    slot.PricePerKwh,
		ChargingSource: slot.ChargingSource,
		IsCharging:     slot.ChargePower > 0,
		IsDischarging:  slot.ChargePower < 0,
		IsIdle:         slot.ChargePower == 0,
		Found:          true,
	})
}

func (api *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		// Schedule slot endpoints
		r.Get("/schedule/current", api.currentScheduleSlot)
		r.Get(fmt.Sprintf("/schedule/{start:%s}", api.TimePattern), api.scheduleSlots)
		r.Get(fmt.Sprintf("/schedule/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.scheduleSlots)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

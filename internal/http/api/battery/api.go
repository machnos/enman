package battery

import (
	"enman/internal/domain"
	"enman/internal/http/api"
	"enman/internal/log"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"time"
)

const (
	errorCodeBatteryRoot            = "-battery"
	errorCodeStartDateParseError    = errorCodeBatteryRoot + "-01"
	errorCodeEndDateParseError      = errorCodeBatteryRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeBatteryRoot + "-03"
	errorCodeUnableToLoadStates     = errorCodeBatteryRoot + "-04"
	errorCodeUnableToLoadSources    = errorCodeBatteryRoot + "-07"
)

type Api struct {
	*api.BaseApi
}

func NewApi(system *domain.System, repository domain.Repository) *Api {
	return &Api{
		api.NewBaseApi(system, repository),
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
	sources, err := api.Repository.ElectricitySourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (api *Api) states(w http.ResponseWriter, r *http.Request) {
	type state struct {
		Current float32 `json:"current"`
		Voltage float32 `json:"voltage"`
		Power   float32 `json:"power"`
		SoC     float32 `json:"soc"`
		SoH     float32 `json:"soh"`
	}

	type bucket struct {
		StartTime time.Time         `json:"start_time"`
		EndTime   time.Time         `json:"end_time"`
		States    map[string]*state `json:"states"`
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
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitMinute,
		WindowAmount: 1,
		Functions:    []domain.AggregateFunction{domain.AggregateFunctionMean},
		CreateEmpty:  false,
	}
	statesRecords, err := api.Repository.BatteryStates(
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
	for _, statesRecord := range statesRecords {
		if rsp.Sources[statesRecord.Name] == nil {
			rsp.Sources[statesRecord.Name] = &source{Role: statesRecord.Role}
		}
		b := &bucket{
			StartTime: statesRecord.StartTime,
			EndTime:   statesRecord.EndTime,
			States:    make(map[string]*state),
		}
		for _, fn := range aggregate.Functions {
			b.States[fn.String()] = &state{
				Current: statesRecord.States[fn].Current(),
				Voltage: statesRecord.States[fn].Voltage(),
				Power:   statesRecord.States[fn].Power(),
				SoC:     statesRecord.States[fn].SoC(),
				SoH:     statesRecord.States[fn].SoH(),
			}
		}
		rsp.Sources[statesRecord.Name].Buckets = append(rsp.Sources[statesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
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
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

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
	errorCodeUnableToLoadSources    = errorCodeBatteryRoot + "-05"
)

type BatteryApi struct {
	*api.BaseApi
}

func NewBatteryApi(system *domain.System, repository domain.Repository) *BatteryApi {
	return &BatteryApi{
		api.NewBaseApi(system, repository),
	}
}

func (b *BatteryApi) sources(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Sources []string `json:"sources"`
	}{}
	startTime, endTime, success := b.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	sources, err := b.Repository.ElectricitySourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		b.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (b *BatteryApi) states(w http.ResponseWriter, r *http.Request) {
	type stateResponse struct {
		Time    time.Time `json:"time"`
		Current float32   `json:"current"`
		Voltage float32   `json:"voltage"`
		Power   float32   `json:"power"`
		SoC     float32   `json:"soc"`
		SoH     float32   `json:"soh"`
	}
	type stateSerie struct {
		Role   string          `json:"role"`
		States []stateResponse `json:"states"`
	}
	type batteryStatesResponse struct {
		States map[string]*stateSerie `json:"states"`
	}
	rsp := batteryStatesResponse{
		States: make(map[string]*stateSerie),
	}
	startTime, endTime, success := b.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitMinute,
		WindowAmount: 1,
		Function:     domain.Mean{},
		CreateEmpty:  false,
	}
	states, err := b.Repository.BatteryStates(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		b.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)

	if err != nil {
		log.Error(err.Error())
		b.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadStates, err.Error())
		return
	}
	for _, state := range states {
		if rsp.States[state.Name] == nil {
			rsp.States[state.Name] = &stateSerie{Role: state.Role}
		}
		rsp.States[state.Name].States = append(rsp.States[state.Name].States, stateResponse{
			Time:    state.Time,
			Current: state.Current(),
			Voltage: state.Voltage(),
			Power:   state.Power(),
			SoC:     state.SoC(),
			SoH:     state.SoH(),
		})
	}
	render.JSON(w, r, rsp)
}

func (b *BatteryApi) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", b.TimePattern), b.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.sources)
		r.Get(fmt.Sprintf("/states/{start:%s}", b.TimePattern), b.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", b.TimePattern), b.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.states)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

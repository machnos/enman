package energy_flow

import (
	"enman/internal"
	"enman/internal/http/api"
	"enman/internal/log"
	"enman/internal/persistency"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"time"
)

const (
	errorCodeEnergyFlowRoot         = "-energy_flow"
	errorCodeStartDateParseError    = errorCodeEnergyFlowRoot + "-01"
	errorCodeEndDateParseError      = errorCodeEnergyFlowRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeEnergyFlowRoot + "-03"
	errorCodeUnableToLoadTotals     = errorCodeEnergyFlowRoot + "-04"
	errorCodeUnableToLoadNames      = errorCodeEnergyFlowRoot + "-05"
)

type EnergyFlowApi struct {
	*api.BaseApi
}

func NewEnergyFlowApi(system *internal.System, repository persistency.Repository) *EnergyFlowApi {
	return &EnergyFlowApi{
		api.NewBaseApi(system, repository),
	}
}

func (e *EnergyFlowApi) validateStartAndEndParams(w http.ResponseWriter, r *http.Request) (*time.Time, *time.Time, bool) {
	startTime, err := time.ParseInLocation("2006-01-02", chi.URLParam(r, "start"), e.System.Location())
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusBadRequest, errorCodeStartDateParseError, "Unable to parse start date")
		return nil, nil, false
	}
	endTime := time.Now()
	if chi.URLParam(r, "end") != "" {
		parsedTime, err := time.ParseInLocation("2006-01-02", chi.URLParam(r, "end"), e.System.Location())
		if err != nil {
			log.Error(err.Error())
			e.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateParseError, "Unable to parse end date")
			return nil, nil, false
		}
		endTime = parsedTime
	}
	if endTime.Before(startTime) {
		e.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateBeforeStartDate, "End date is before start date")
		return nil, nil, false
	}
	return &startTime, &endTime, true

}

func (e *EnergyFlowApi) names(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Names []string `json:"names"`
	}{}
	names, err := e.Repository.EnergyFlowNames()
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadNames, err.Error())
		return
	}
	rsp.Names = names
	render.JSON(w, r, rsp)
}

func (e *EnergyFlowApi) usage(w http.ResponseWriter, r *http.Request) {
	type usageResponse struct {
		Time                time.Time `json:"time"`
		TotalEnergyConsumed float64   `json:"total_energy_consumed"`
		TotalEnergyProvided float64   `json:"total_energy_provided"`
	}
	type usageSerie struct {
		Role   string          `json:"role"`
		Usages []usageResponse `json:"usages"`
	}
	type usagesResponse struct {
		Usages map[string]*usageSerie `json:"usages"`
	}
	rsp := usagesResponse{
		Usages: make(map[string]*usageSerie),
	}
	startTime, endTime, success := e.validateStartAndEndParams(w, r)
	if !success {
		return
	}
	aggregate := &persistency.AggregateConfiguration{
		WindowUnit:   persistency.Hour,
		WindowAmount: 1,
		Function:     persistency.Mean{},
		CreateEmpty:  false,
	}
	usages, err := e.Repository.EnergyFlowUsages(startTime, endTime, chi.URLParam(r, "name"), aggregate)
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadTotals, err.Error())
		return
	}
	for _, usage := range usages {
		if rsp.Usages[usage.Name] == nil {
			rsp.Usages[usage.Name] = &usageSerie{Role: usage.Role}
		}
		rsp.Usages[usage.Name].Usages = append(rsp.Usages[usage.Name].Usages, usageResponse{
			Time:                usage.Time,
			TotalEnergyConsumed: usage.TotalEnergyConsumed(),
			TotalEnergyProvided: usage.TotalEnergyProvided(),
		})
	}
	render.JSON(w, r, rsp)
}

func (e *EnergyFlowApi) states(w http.ResponseWriter, r *http.Request) {
	type LineValues struct {
		L1 float32 `json:"l1"`
		L2 float32 `json:"l2"`
		L3 float32 `json:"l3"`
	}
	type stateResponse struct {
		Time         time.Time  `json:"time"`
		Current      LineValues `json:"current"`
		TotalCurrent float32    `json:"total_current"`
		Voltage      LineValues `json:"voltage"`
		Power        LineValues `json:"power"`
		TotalPower   float32    `json:"total_power"`
	}
	type stateSerie struct {
		Role   string          `json:"role"`
		States []stateResponse `json:"states"`
	}
	type energyStatesResponse struct {
		States map[string]*stateSerie `json:"states"`
	}
	rsp := energyStatesResponse{
		States: make(map[string]*stateSerie),
	}
	startTime, endTime, success := e.validateStartAndEndParams(w, r)
	if !success {
		return
	}
	aggregate := &persistency.AggregateConfiguration{
		WindowUnit:   persistency.Minute,
		WindowAmount: 1,
		Function:     persistency.Mean{},
		CreateEmpty:  false,
	}
	states, err := e.Repository.EnergyFlowStates(startTime, endTime, chi.URLParam(r, "name"), aggregate)
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadTotals, err.Error())
		return
	}
	for _, state := range states {
		if rsp.States[state.Name] == nil {
			rsp.States[state.Name] = &stateSerie{Role: state.Role}
		}
		rsp.States[state.Name].States = append(rsp.States[state.Name].States, stateResponse{
			Time: state.Time,
			Current: LineValues{
				L1: state.Current(0),
				L2: state.Current(1),
				L3: state.Current(2),
			},
			TotalCurrent: state.TotalCurrent(),
			Voltage: LineValues{
				L1: state.Voltage(0),
				L2: state.Voltage(1),
				L3: state.Voltage(2),
			},
			Power: LineValues{
				L1: state.Power(0),
				L2: state.Power(1),
				L3: state.Power(2),
			},
			TotalPower: state.TotalPower(),
		})
	}
	render.JSON(w, r, rsp)
}

func (e *EnergyFlowApi) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get("/names", e.names)
		r.Get(fmt.Sprintf("/usages/{start:%s}", e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/states/{start:%s}", e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/{name}/usage/{start:%s}", e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/{name}/usage/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/{name}/states/{start:%s}", e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/{name}/states/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.states)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

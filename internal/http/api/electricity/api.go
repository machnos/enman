package electricity

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
	errorCodeElectricityRoot        = "-electricity"
	errorCodeStartDateParseError    = errorCodeElectricityRoot + "-01"
	errorCodeEndDateParseError      = errorCodeElectricityRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeElectricityRoot + "-03"
	errorCodeUnableToLoadStates     = errorCodeElectricityRoot + "-04"
	errorCodeUnableToLoadUsages     = errorCodeElectricityRoot + "-05"
	errorCodeUnableToLoadCosts      = errorCodeElectricityRoot + "-06"
	errorCodeUnableToLoadSources    = errorCodeElectricityRoot + "-07"
)

type ElectricityApi struct {
	*api.BaseApi
}

func NewElectricityApi(system *domain.System, repository domain.Repository) *ElectricityApi {
	return &ElectricityApi{
		api.NewBaseApi(system, repository),
	}
}

func (e *ElectricityApi) sources(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Sources []string `json:"sources"`
	}{}
	startTime, endTime, success := e.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	sources, err := e.Repository.ElectricitySourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (e *ElectricityApi) usage(w http.ResponseWriter, r *http.Request) {
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
	startTime, endTime, success := e.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Function:     domain.Max{},
		CreateEmpty:  false,
	}
	usages, err := e.Repository.ElectricityUsages(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		e.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadUsages, err.Error())
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

func (e *ElectricityApi) states(w http.ResponseWriter, r *http.Request) {
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
	startTime, endTime, success := e.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitMinute,
		WindowAmount: 1,
		Function:     domain.Mean{},
		CreateEmpty:  false,
	}
	states, err := e.Repository.ElectricityStates(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		e.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)

	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadStates, err.Error())
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

func (e *ElectricityApi) costs(w http.ResponseWriter, r *http.Request) {
	type costResponse struct {
		Time              time.Time `json:"time"`
		ConsumptionCosts  float32   `json:"consumption_costs"`
		ConsumptionEnergy float32   `json:"consumption_energy"`
		FeedbackCosts     float32   `json:"feedback_costs"`
		FeedbackEnergy    float32   `json:"feedback_energy"`
		NetCosts          float32   `json:"net_costs"`
	}
	type costsResponse struct {
		Costs map[string][]*costResponse `json:"costs"`
	}
	rsp := costsResponse{
		Costs: make(map[string][]*costResponse),
	}
	startTime, endTime, success := e.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Function:     domain.Sum{},
		CreateEmpty:  false,
	}
	costs, err := e.Repository.ElectricityCosts(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		e.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)
	if err != nil {
		log.Error(err.Error())
		e.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadCosts, err.Error())
		return
	}
	for _, cost := range costs {
		rsp.Costs[cost.Name] = append(rsp.Costs[cost.Name], &costResponse{
			Time:              cost.Time,
			ConsumptionCosts:  cost.ConsumptionCosts,
			ConsumptionEnergy: cost.ConsumptionEnergy,
			FeedbackCosts:     cost.FeedbackCosts,
			FeedbackEnergy:    cost.FeedbackEnergy,
			NetCosts:          cost.ConsumptionCosts - cost.FeedbackCosts,
		})
	}
	render.JSON(w, r, rsp)

}

func (e *ElectricityApi) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", e.TimePattern), e.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.sources)
		r.Get(fmt.Sprintf("/usages/{start:%s}", e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/states/{start:%s}", e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/costs/{start:%s}", e.TimePattern), e.costs)
		r.Get(fmt.Sprintf("/costs/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.costs)
		r.Get(fmt.Sprintf("/{sourceName}/usage/{start:%s}", e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/{sourceName}/usage/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.usage)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.states)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}", e.TimePattern), e.costs)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}/{end:%s}", e.TimePattern, e.TimePattern), e.costs)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

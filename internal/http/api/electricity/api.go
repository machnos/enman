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

func (api *Api) usage(w http.ResponseWriter, r *http.Request) {
	type usage struct {
		TotalEnergyConsumed float64 `json:"total_energy_consumed"`
		TotalEnergyProvided float64 `json:"total_energy_provided"`
	}
	type bucket struct {
		StartTime time.Time         `json:"start_time"`
		EndTime   time.Time         `json:"end_time"`
		Usages    map[string]*usage `json:"usages"`
	}
	type source struct {
		Role    string    `json:"role"`
		Buckets []*bucket `json:"buckets"`
	}
	type usagesResponse struct {
		Sources map[string]*source `json:"sources"`
	}
	rsp := usagesResponse{
		Sources: make(map[string]*source),
	}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []domain.AggregateFunction{domain.AggregateFunctionMin, domain.AggregateFunctionMax},
		CreateEmpty:  false,
	}
	usagesRecords, err := api.Repository.ElectricityUsages(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		api.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadUsages, err.Error())
		return
	}
	for _, usagesRecord := range usagesRecords {
		if rsp.Sources[usagesRecord.Name] == nil {
			rsp.Sources[usagesRecord.Name] = &source{Role: usagesRecord.Role}
		}
		b := &bucket{
			StartTime: usagesRecord.StartTime,
			EndTime:   usagesRecord.EndTime,
			Usages:    make(map[string]*usage),
		}
		for _, fn := range aggregate.Functions {
			b.Usages[fn.String()] = &usage{
				TotalEnergyConsumed: usagesRecord.Usages[fn].TotalEnergyConsumed(),
				TotalEnergyProvided: usagesRecord.Usages[fn].TotalEnergyProvided(),
			}
		}
		rsp.Sources[usagesRecord.Name].Buckets = append(rsp.Sources[usagesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
}

func (api *Api) states(w http.ResponseWriter, r *http.Request) {
	type lineValues struct {
		L1 float32 `json:"l1"`
		L2 float32 `json:"l2"`
		L3 float32 `json:"l3"`
	}
	type state struct {
		Current      *lineValues `json:"current"`
		TotalCurrent float32     `json:"total_current"`
		Voltage      *lineValues `json:"voltage"`
		Power        *lineValues `json:"power"`
		TotalPower   float32     `json:"total_power"`
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
	type energyStatesResponse struct {
		Sources map[string]*source `json:"sources"`
	}
	rsp := energyStatesResponse{
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
	statesRecords, err := api.Repository.ElectricityStates(
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
				Current: &lineValues{
					L1: statesRecord.States[fn].Current(0),
					L2: statesRecord.States[fn].Current(1),
					L3: statesRecord.States[fn].Current(2),
				},
				TotalCurrent: statesRecord.States[fn].TotalCurrent(),
				Voltage: &lineValues{
					L1: statesRecord.States[fn].Voltage(0),
					L2: statesRecord.States[fn].Voltage(1),
					L3: statesRecord.States[fn].Voltage(2),
				},
				Power: &lineValues{
					L1: statesRecord.States[fn].Power(0),
					L2: statesRecord.States[fn].Power(1),
					L3: statesRecord.States[fn].Power(2),
				},
				TotalPower: statesRecord.States[fn].TotalPower()}
		}
		rsp.Sources[statesRecord.Name].Buckets = append(rsp.Sources[statesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
}

func (api *Api) costs(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		StartTime         time.Time `json:"start_time"`
		EndTime           time.Time `json:"end_time"`
		ConsumptionCosts  float32   `json:"consumption_costs"`
		ConsumptionEnergy float32   `json:"consumption_energy"`
		FeedbackCosts     float32   `json:"feedback_costs"`
		FeedbackEnergy    float32   `json:"feedback_energy"`
		NetCosts          float32   `json:"net_costs"`
	}
	type costsResponse struct {
		Sources map[string][]*bucket `json:"sources"`
	}
	rsp := costsResponse{
		Sources: make(map[string][]*bucket),
	}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []domain.AggregateFunction{domain.AggregateFunctionSum},
		CreateEmpty:  false,
	}
	costs, err := api.Repository.ElectricityCosts(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		api.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadCosts, err.Error())
		return
	}
	for _, cost := range costs {
		rsp.Sources[cost.Name] = append(rsp.Sources[cost.Name], &bucket{
			StartTime:         cost.StartTime,
			EndTime:           cost.EndTime,
			ConsumptionCosts:  cost.ConsumptionCosts,
			ConsumptionEnergy: cost.ConsumptionEnergy,
			FeedbackCosts:     cost.FeedbackCosts,
			FeedbackEnergy:    cost.FeedbackEnergy,
			NetCosts:          cost.ConsumptionCosts - cost.FeedbackCosts,
		})
	}
	render.JSON(w, r, rsp)

}

func (api *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/usages/{start:%s}", api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}", api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

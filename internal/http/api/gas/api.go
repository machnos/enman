package gas

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
	errorCodeGasRoot                = "-gas"
	errorCodeStartDateParseError    = errorCodeGasRoot + "-01"
	errorCodeEndDateParseError      = errorCodeGasRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeGasRoot + "-03"
	errorCodeUnableToLoadUsages     = errorCodeGasRoot + "-05"
	errorCodeUnableToLoadCosts      = errorCodeGasRoot + "-06"
	errorCodeUnableToLoadSources    = errorCodeGasRoot + "-07"
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
	sources, err := api.Repository.GasSourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (api *Api) usages(w http.ResponseWriter, r *http.Request) {
	type usage struct {
		GasConsumed float64 `json:"gas_consumed"`
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
	usagesRecords, err := api.Repository.GasUsages(
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
			Usages:    map[string]*usage{},
		}
		for _, fn := range aggregate.Functions {
			b.Usages[fn.String()] = &usage{
				GasConsumed: usagesRecord.Usages[fn].GasConsumed(),
			}
		}
		rsp.Sources[usagesRecord.Name].Buckets = append(rsp.Sources[usagesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
}

func (api *Api) costs(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		StartTime        time.Time `json:"start_time"`
		EndTime          time.Time `json:"end_time"`
		ConsumptionCosts float32   `json:"consumption_costs"`
		ConsumptionUsage float32   `json:"consumption_usage"`
		NetCosts         float32   `json:"net_costs"`
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
	costs, err := api.Repository.GasCosts(
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
			StartTime:        cost.StartTime,
			EndTime:          cost.EndTime,
			ConsumptionCosts: cost.ConsumptionCosts,
			ConsumptionUsage: cost.ConsumptionUsage,
			NetCosts:         cost.ConsumptionCosts,
		})
	}
	render.JSON(w, r, rsp)
}

func (api *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/usages/{start:%s}", api.TimePattern), api.usages)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usages)
		r.Get(fmt.Sprintf("/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}", api.TimePattern), api.usages)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usages)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

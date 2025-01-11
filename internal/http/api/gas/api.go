package gas

import (
	"enman/internal/domain"
	"enman/internal/domain/repository"
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
	repository repository.Gas
}

func NewApi(system *domain.System, repository repository.Gas) *Api {
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
	sources, err := api.repository.GasSourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (api *Api) usages(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		StartTime time.Time      `json:"start_time"`
		EndTime   time.Time      `json:"end_time"`
		Usages    map[string]any `json:"usages"`
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
	aggregate := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionMin, repository.AggregateFunctionMax},
		CreateEmpty:  false,
	}
	usagesRecords, err := api.repository.GasUsages(
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
	requestedFields := api.ParseFieldsFromRequestURL(r)
	for _, usagesRecord := range usagesRecords {
		if rsp.Sources[usagesRecord.Name] == nil {
			rsp.Sources[usagesRecord.Name] = &source{Role: usagesRecord.Role}
		}
		b := &bucket{
			StartTime: usagesRecord.StartTime,
			EndTime:   usagesRecord.EndTime,
			Usages:    make(map[string]any),
		}
		for _, fn := range aggregate.Functions {
			usageMap := make(map[string]any)
			api.ConditionallyAddField(requestedFields, "gas_consumed", usagesRecord.Usages[fn].GasConsumed(), usageMap)
			b.Usages[fn.String()] = usageMap
		}
		rsp.Sources[usagesRecord.Name].Buckets = append(rsp.Sources[usagesRecord.Name].Buckets, b)
	}
	render.JSON(w, r, rsp)
}

func (api *Api) costs(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		StartTime time.Time      `json:"start_time"`
		EndTime   time.Time      `json:"end_time"`
		Costs     map[string]any `json:"costs"`
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
	aggregate := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionSum},
		CreateEmpty:  false,
	}
	costsRecords, err := api.repository.GasCosts(
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
	requestedFields := api.ParseFieldsFromRequestURL(r)
	for _, costsRecord := range costsRecords {
		b := &bucket{
			StartTime: costsRecord.StartTime,
			EndTime:   costsRecord.EndTime,
			Costs:     make(map[string]any),
		}
		for _, fn := range aggregate.Functions {
			costsMap := make(map[string]any)
			api.ConditionallyAddField(requestedFields, "consumption_costs", costsRecord.Costs[fn].ConsumptionCosts(), costsMap)
			api.ConditionallyAddField(requestedFields, "consumption_usage", costsRecord.Costs[fn].ConsumptionUsage(), costsMap)
			api.ConditionallyAddField(requestedFields, "net_costs", costsRecord.Costs[fn].NetCosts(), costsMap)
			b.Costs[fn.String()] = costsMap
		}
		rsp.Sources[costsRecord.Name] = append(rsp.Sources[costsRecord.Name], b)
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

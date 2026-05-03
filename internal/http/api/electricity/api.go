package electricity

import (
	"enman/internal/domain"
	"enman/internal/domain/events"
	"enman/internal/http/api"
	"enman/internal/http/api/battery"
	"enman/internal/log"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
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
	errorCodeUnableToLoadForecast   = errorCodeElectricityRoot + "-08"
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
			api.ConditionallyAddField(requestedFields, "total_energy_consumed", usagesRecord.Usages[fn].TotalEnergyConsumed(), usageMap)
			api.ConditionallyAddField(requestedFields, "total_energy_provided", usagesRecord.Usages[fn].TotalEnergyProvided(), usageMap)
			b.Usages[fn.String()] = usageMap
		}
		rsp.Sources[usagesRecord.Name].Buckets = append(rsp.Sources[usagesRecord.Name].Buckets, b)
	}
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
			lineMap := make(map[string]any)
			api.ConditionallyAddField(requestedFields, "current.l1", statesRecord.States[fn].Current(0), lineMap)
			api.ConditionallyAddField(requestedFields, "current.l2", statesRecord.States[fn].Current(1), lineMap)
			api.ConditionallyAddField(requestedFields, "current.l3", statesRecord.States[fn].Current(2), lineMap)
			if len(lineMap) > 0 {
				stateMap["current"] = lineMap
			}
			api.ConditionallyAddField(requestedFields, "total_current", statesRecord.States[fn].TotalCurrent(), stateMap)
			lineMap = make(map[string]any)
			api.ConditionallyAddField(requestedFields, "voltage.l1", statesRecord.States[fn].Voltage(0), lineMap)
			api.ConditionallyAddField(requestedFields, "voltage.l2", statesRecord.States[fn].Voltage(1), lineMap)
			api.ConditionallyAddField(requestedFields, "voltage.l3", statesRecord.States[fn].Voltage(2), lineMap)
			if len(lineMap) > 0 {
				stateMap["voltage"] = lineMap
			}
			lineMap = make(map[string]any)
			api.ConditionallyAddField(requestedFields, "power.l1", statesRecord.States[fn].Power(0), lineMap)
			api.ConditionallyAddField(requestedFields, "power.l2", statesRecord.States[fn].Power(1), lineMap)
			api.ConditionallyAddField(requestedFields, "power.l3", statesRecord.States[fn].Power(2), lineMap)
			if len(lineMap) > 0 {
				stateMap["power"] = lineMap
			}
			api.ConditionallyAddField(requestedFields, "total_power", statesRecord.States[fn].TotalPower(), stateMap)
			b.States[fn.String()] = stateMap
		}
		rsp.Sources[statesRecord.Name].Buckets = append(rsp.Sources[statesRecord.Name].Buckets, b)
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
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []domain.AggregateFunction{domain.AggregateFunctionSum},
		CreateEmpty:  false,
	}
	costsRecords, err := api.Repository.ElectricityCosts(
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
			api.ConditionallyAddField(requestedFields, "consumption_energy", costsRecord.Costs[fn].ConsumptionEnergy(), costsMap)
			api.ConditionallyAddField(requestedFields, "feedback_costs", costsRecord.Costs[fn].FeedbackCosts(), costsMap)
			api.ConditionallyAddField(requestedFields, "feedback_energy", costsRecord.Costs[fn].FeedbackEnergy(), costsMap)
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
		r.Get(fmt.Sprintf("/usages/{start:%s}", api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/forecast/{start:%s}", api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/forecast/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}", api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.usage)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}", api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/costs/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.costs)
		r.Get(fmt.Sprintf("/{sourceName}/forecast/{start:%s}", api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/{sourceName}/forecast/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.forecast)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

// forecast returns the per-source load and pv forecasts.
func (api *Api) forecast(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, ok := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !ok {
		return
	}
	sourceName := chi.URLParam(r, "sourceName")
	loadRecords, err := api.Repository.Forecasts(startTime, endTime, "", string(events.ForecastKindLoad), sourceName)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadForecast, err.Error())
		return
	}
	pvRecords, err := api.Repository.Forecasts(startTime, endTime, "", string(events.ForecastKindPv), sourceName)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadForecast, err.Error())
		return
	}
	render.JSON(w, r, battery.ForecastResponseFromRecords(append(loadRecords, pvRecords...)))
}

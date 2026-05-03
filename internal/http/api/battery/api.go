package battery

import (
	"enman/internal/domain"
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
	errorCodeUnableToLoadSources    = errorCodeBatteryRoot + "-07"
	errorCodeUnableToLoadSchedule   = errorCodeBatteryRoot + "-08"
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

func (api *Api) schedule(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		BucketStart  time.Time `json:"bucket_start"`
		BucketSize   int64     `json:"bucket_size_seconds"`
		GeneratedAt  time.Time `json:"generated_at"`
		Action       string    `json:"action"`
		PowerW       float32   `json:"power_w"`
		PredictedSoC float32   `json:"predicted_soc"`
		Reason       string    `json:"reason"`
	}
	type batteryGroup struct {
		Buckets []*bucket `json:"buckets"`
	}
	rsp := struct {
		Batteries map[string]*batteryGroup `json:"batteries"`
	}{Batteries: make(map[string]*batteryGroup)}

	startTime, endTime, ok := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !ok {
		return
	}
	records, err := api.Repository.BatterySchedules(startTime, endTime, chi.URLParam(r, "batteryName"))
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSchedule, err.Error())
		return
	}
	for _, rec := range records {
		group, exists := rsp.Batteries[rec.BatteryName]
		if !exists {
			group = &batteryGroup{}
			rsp.Batteries[rec.BatteryName] = group
		}
		group.Buckets = append(group.Buckets, &bucket{
			BucketStart:  rec.BucketStart,
			BucketSize:   int64(rec.BucketSize.Seconds()),
			GeneratedAt:  rec.GeneratedAt,
			Action:       rec.Action,
			PowerW:       rec.PowerW,
			PredictedSoC: rec.PredictedSoC,
			Reason:       rec.Reason,
		})
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
		r.Get(fmt.Sprintf("/schedule/{start:%s}", api.TimePattern), api.schedule)
		r.Get(fmt.Sprintf("/schedule/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.schedule)
		r.Get(fmt.Sprintf("/{batteryName}/schedule/{start:%s}", api.TimePattern), api.schedule)
		r.Get(fmt.Sprintf("/{batteryName}/schedule/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.schedule)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

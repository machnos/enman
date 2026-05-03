package battery

import (
	"enman/internal/domain"
	"enman/internal/domain/events"
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
	errorCodeUnableToLoadForecast   = errorCodeBatteryRoot + "-08"
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

func (api *Api) forecast(w http.ResponseWriter, r *http.Request) {
	startTime, endTime, ok := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !ok {
		return
	}
	records, err := api.Repository.Forecasts(startTime, endTime, "", string(events.ForecastKindBattery), chi.URLParam(r, "sourceName"))
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadForecast, err.Error())
		return
	}
	render.JSON(w, r, ForecastResponseFromRecords(records))
}

// ForecastBucket is the per-source per-bucket forecast row exposed by /forecast.
type ForecastBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	BucketSize  int64     `json:"bucket_size_seconds"`
	GeneratedAt time.Time `json:"generated_at"`
	Watts       float32   `json:"watts"`
	Confidence  float32   `json:"confidence"`
}

// ForecastSource groups forecast buckets for a single source.
type ForecastSource struct {
	Kind    string            `json:"kind"`
	Buckets []*ForecastBucket `json:"buckets"`
}

// ForecastResponse mirrors the {Sources -> Buckets} shape used by states/usages.
type ForecastResponse struct {
	Sources map[string]*ForecastSource `json:"sources"`
}

// ForecastResponseFromRecords groups forecast records into the API response
// shape. Exported so other domain APIs (electricity, gas) can reuse it.
func ForecastResponseFromRecords(records []*domain.ForecastRecord) ForecastResponse {
	rsp := ForecastResponse{Sources: make(map[string]*ForecastSource)}
	for _, rec := range records {
		if rec == nil {
			continue
		}
		group, exists := rsp.Sources[rec.SourceName]
		if !exists {
			group = &ForecastSource{Kind: rec.Kind}
			rsp.Sources[rec.SourceName] = group
		}
		group.Buckets = append(group.Buckets, &ForecastBucket{
			BucketStart: rec.BucketStart,
			BucketSize:  int64(rec.BucketSize.Seconds()),
			GeneratedAt: rec.GeneratedAt,
			Watts:       rec.Watts,
			Confidence:  rec.Confidence,
		})
	}
	return rsp
}

func (api *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.sources)
		r.Get(fmt.Sprintf("/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/forecast/{start:%s}", api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/forecast/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}", api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/states/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.states)
		r.Get(fmt.Sprintf("/{sourceName}/forecast/{start:%s}", api.TimePattern), api.forecast)
		r.Get(fmt.Sprintf("/{sourceName}/forecast/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.forecast)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

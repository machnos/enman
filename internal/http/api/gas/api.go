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
	errorCodeBatteryRoot            = "-gas"
	errorCodeStartDateParseError    = errorCodeBatteryRoot + "-01"
	errorCodeEndDateParseError      = errorCodeBatteryRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeBatteryRoot + "-03"
	errorCodeUnableToLoadUsages     = errorCodeBatteryRoot + "-04"
	errorCodeUnableToLoadSources    = errorCodeBatteryRoot + "-05"
)

type Api struct {
	*api.BaseApi
}

func NewApi(system *domain.System, repository domain.Repository) *Api {
	return &Api{
		api.NewBaseApi(system, repository),
	}
}

func (b *Api) sources(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Sources []string `json:"sources"`
	}{}
	startTime, endTime, success := b.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	sources, err := b.Repository.GasSourceNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		b.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadSources, err.Error())
		return
	}
	rsp.Sources = sources
	render.JSON(w, r, rsp)
}

func (b *Api) usages(w http.ResponseWriter, r *http.Request) {
	type usageResponse struct {
		Time        time.Time `json:"time"`
		GasConsumed float64   `json:"gas_consumed"`
	}
	type usageSerie struct {
		Role   string          `json:"role"`
		Usages []usageResponse `json:"usages"`
	}
	type gasUsagesResponse struct {
		Usages map[string]*usageSerie `json:"usages"`
	}
	rsp := gasUsagesResponse{
		Usages: make(map[string]*usageSerie),
	}
	startTime, endTime, success := b.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	aggregate := &domain.AggregateConfiguration{
		WindowUnit:   domain.WindowUnitHour,
		WindowAmount: 1,
		Function:     domain.Min{},
		CreateEmpty:  false,
	}
	usages, err := b.Repository.GasUsages(
		startTime,
		endTime,
		chi.URLParam(r, "sourceName"),
		b.ParseAggregateConfigurationFromRequestURL(r, aggregate),
	)

	if err != nil {
		log.Error(err.Error())
		b.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadUsages, err.Error())
		return
	}
	for _, usage := range usages {
		if rsp.Usages[usage.Name] == nil {
			rsp.Usages[usage.Name] = &usageSerie{Role: usage.Role}
		}
		rsp.Usages[usage.Name].Usages = append(rsp.Usages[usage.Name].Usages, usageResponse{
			Time:        usage.Time,
			GasConsumed: usage.GasConsumed(),
		})
	}
	render.JSON(w, r, rsp)
}

func (b *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/sources/{start:%s}", b.TimePattern), b.sources)
		r.Get(fmt.Sprintf("/sources/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.sources)
		r.Get(fmt.Sprintf("/usages/{start:%s}", b.TimePattern), b.usages)
		r.Get(fmt.Sprintf("/usages/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.usages)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}", b.TimePattern), b.usages)
		r.Get(fmt.Sprintf("/{sourceName}/usages/{start:%s}/{end:%s}", b.TimePattern, b.TimePattern), b.usages)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

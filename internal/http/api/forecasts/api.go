package forecasts

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
	errorCodeForecastsRoot          = "-forecasts"
	errorCodeStartDateParseError    = errorCodeForecastsRoot + "-01"
	errorCodeEndDateParseError      = errorCodeForecastsRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodeForecastsRoot + "-03"
	errorCodeUnableToLoadForecasts  = errorCodeForecastsRoot + "-04"
)

type Api struct {
	*api.BaseApi
}

func NewApi(system *domain.System, repository domain.Repository) *Api {
	return &Api{api.NewBaseApi(system, repository)}
}

func (a *Api) list(w http.ResponseWriter, r *http.Request) {
	type bucket struct {
		BucketStart time.Time `json:"bucket_start"`
		BucketSize  int64     `json:"bucket_size_seconds"`
		GeneratedAt time.Time `json:"generated_at"`
		Watts       float32   `json:"watts"`
		Confidence  float32   `json:"confidence"`
	}
	type kindGroup struct {
		Buckets []*bucket `json:"buckets"`
	}
	rsp := struct {
		Forecasts map[string]*kindGroup `json:"forecasts"`
	}{Forecasts: make(map[string]*kindGroup)}

	startTime, endTime, ok := a.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !ok {
		return
	}
	model := r.URL.Query().Get("model")
	kind := chi.URLParam(r, "kind")
	if kind == "" {
		kind = r.URL.Query().Get("kind")
	}
	records, err := a.Repository.Forecasts(startTime, endTime, model, kind)
	if err != nil {
		log.Error(err.Error())
		a.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadForecasts, err.Error())
		return
	}
	for _, rec := range records {
		key := rec.Kind
		if rec.ModelName != "" {
			key = rec.Kind + "/" + rec.ModelName
		}
		group, exists := rsp.Forecasts[key]
		if !exists {
			group = &kindGroup{}
			rsp.Forecasts[key] = group
		}
		group.Buckets = append(group.Buckets, &bucket{
			BucketStart: rec.BucketStart,
			BucketSize:  int64(rec.BucketSize.Seconds()),
			GeneratedAt: rec.GeneratedAt,
			Watts:       rec.Watts,
			Confidence:  rec.Confidence,
		})
	}
	render.JSON(w, r, rsp)
}

func (a *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/{start:%s}", a.TimePattern), a.list)
		r.Get(fmt.Sprintf("/{start:%s}/{end:%s}", a.TimePattern, a.TimePattern), a.list)
		r.Get(fmt.Sprintf("/{kind}/{start:%s}", a.TimePattern), a.list)
		r.Get(fmt.Sprintf("/{kind}/{start:%s}/{end:%s}", a.TimePattern, a.TimePattern), a.list)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

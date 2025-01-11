package api

import (
	"enman/internal/domain"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	errorCodeRoot = "api"
)

type Error struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type Api interface {
	Router(map[string]func(r chi.Router)) func(r chi.Router)
}

type BaseApi struct {
	System      *domain.System
	TimePattern string
}

func NewBaseApi(system *domain.System) *BaseApi {
	return &BaseApi{
		System:      system,
		TimePattern: "^\\d{4}-\\d{2}-\\d{2}(T(\\d{2}|\\d{2}:\\d{2}))?$",
	}
}

func (b *BaseApi) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.NoCache)
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte("{\"pong\":\"Allan Alcorn\"}"))
		})
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

func (b *BaseApi) ApiError(w http.ResponseWriter, r *http.Request, httpStatusCode int, code string, reason string) {
	render.Status(r, httpStatusCode)
	render.JSON(w, r, Error{
		Code:   errorCodeRoot + code,
		Reason: reason,
	})
}

func (b *BaseApi) ParseTimeFromRequestURL(r *http.Request, urlParamName string, location *time.Location) (time.Time, time.Duration, error) {
	param := chi.URLParam(r, urlParamName)
	if param == "" {
		return time.Time{}, 0, fmt.Errorf("param %s not found in request url", urlParamName)
	}
	match, _ := regexp.MatchString("^\\d{4}-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly, param, location)
		return parsedTime, time.Hour, err
	}
	match, _ = regexp.MatchString("^\\d{4}-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])T(0[0-9]|1[0-9]|2[0-3])$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly+"T15", param, location)
		return parsedTime, time.Minute, err
	}
	match, _ = regexp.MatchString("^\\d{4}-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly+"T15:04", param, location)
		return parsedTime, time.Second, err
	}
	match, _ = regexp.MatchString("^\\d{4}-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly+"T"+time.TimeOnly, param, location)
		return parsedTime, time.Millisecond, err
	}
	return time.Time{}, 0, fmt.Errorf("unable to parse time %s", param)
}

func (b *BaseApi) TruncateToEnd(moment time.Time, duration time.Duration) time.Time {
	switch duration {
	case time.Nanosecond:
		nanos := moment.Nanosecond()
		micros := int(time.Microsecond)
		nanos = nanos/micros*micros + micros - 1
		return time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(), moment.Minute(), moment.Second(), nanos, moment.Location())
	case time.Microsecond:
		nanos := moment.Nanosecond()
		millis := int(time.Millisecond)
		nanos = nanos/millis*millis + millis - 1
		return time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(), moment.Minute(), moment.Second(), nanos, moment.Location())
	case time.Millisecond:
		return time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(), moment.Minute(), moment.Second(), 999999999, moment.Location())
	case time.Second:
		return time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(), moment.Minute(), 59, 999999999, moment.Location())
	case time.Minute:
		return time.Date(moment.Year(), moment.Month(), moment.Day(), moment.Hour(), 59, 59, 999999999, moment.Location())
	case time.Hour:
		return time.Date(moment.Year(), moment.Month(), moment.Day(), 23, 59, 59, 999999999, moment.Location())
	}
	return time.Time{}
}

func (b *BaseApi) ValidateStartAndEndParams(w http.ResponseWriter, r *http.Request, errorCodeStartDateParseError string, errorCodeEndDateParseError string, errorCodeEndDateBeforeStartDate string) (time.Time, time.Time, bool) {
	var truncatedTo time.Duration
	startTime, _, err := b.ParseTimeFromRequestURL(r, "start", b.System.Location())
	if err != nil {
		log.Error(err.Error())
		b.ApiError(w, r, http.StatusBadRequest, errorCodeStartDateParseError, "Unable to parse start date")
		return time.Time{}, time.Time{}, false
	}
	endTime := time.Time{}
	if chi.URLParam(r, "end") != "" {
		endTime, truncatedTo, err = b.ParseTimeFromRequestURL(r, "end", b.System.Location())
		if err != nil {
			log.Error(err.Error())
			b.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateParseError, "Unable to parse end date")
			return time.Time{}, time.Time{}, false
		}
	}
	if !endTime.IsZero() {
		endTime = b.TruncateToEnd(endTime, truncatedTo)
		if endTime.Before(startTime) {
			b.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateBeforeStartDate, "End date is before start date")
			return time.Time{}, time.Time{}, false
		}
	}
	return startTime, endTime, true
}

func (b *BaseApi) ParseAggregateConfigurationFromRequestURL(r *http.Request, aggregate *repository.AggregateConfiguration) *repository.AggregateConfiguration {
	q := r.URL.Query()
	if q.Has("aggregate_window_unit") {
		unit, err := repository.ParseWindowUnit(q.Get("aggregate_window_unit"))
		if err == nil {
			aggregate.WindowUnit = unit
		} else {
			log.Warning(err.Error())
		}
	}
	if q.Has("aggregate_window_amount") {
		value, err := strconv.Atoi(q.Get("aggregate_window_amount"))
		if err == nil {
			aggregate.WindowAmount = uint64(value)
		} else {
			log.Warning(err.Error())
		}
	}
	if q.Has("aggregate_create_empty") {
		value, err := strconv.ParseBool(q.Get("aggregate_create_empty"))
		if err == nil {
			aggregate.CreateEmpty = value
		} else {
			log.Warning(err.Error())
		}
	}
	if q.Has("aggregate_functions") {
		function, err := repository.AggregateFunctionsOf(q.Get("aggregate_functions"))
		if err == nil {
			aggregate.Functions = function
		} else {
			log.Warning(err.Error())
		}
	}
	return aggregate
}

func (b *BaseApi) ParseFieldsFromRequestURL(r *http.Request) []string {
	q := r.URL.Query()
	if q.Has("fields") {
		fields := q.Get("fields")
		return strings.Split(fields, ",")
	}
	return nil
}

func (b *BaseApi) ConditionallyAddField(fieldsToAdd []string, field string, value any, target map[string]any) {
	if fieldsToAdd == nil || slices.Contains(fieldsToAdd, field) {
		target[field] = value
	}
}

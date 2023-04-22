package api

import (
	"enman/internal"
	"enman/internal/persistency"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"regexp"
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
	System      *internal.System
	Repository  persistency.Repository
	TimePattern string
}

func NewBaseApi(system *internal.System, repository persistency.Repository) *BaseApi {
	return &BaseApi{
		System:      system,
		Repository:  repository,
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
	match, _ := regexp.MatchString("^\\d{4}\\-(0[1-9]|1[012])\\-(0[1-9]|[12][0-9]|3[01])$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly, param, location)
		return parsedTime, time.Hour, err
	}
	match, _ = regexp.MatchString("^\\d{4}\\-(0[1-9]|1[012])\\-(0[1-9]|[12][0-9]|3[01])T(0[0-9]|1[0-9]|2[0-3])$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly+"T15", param, location)
		return parsedTime, time.Minute, err
	}
	match, _ = regexp.MatchString("^\\d{4}\\-(0[1-9]|1[012])\\-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]$", param)
	if match {
		parsedTime, err := time.ParseInLocation(time.DateOnly+"T15:04", param, location)
		return parsedTime, time.Second, err
	}
	match, _ = regexp.MatchString("^\\d{4}\\-(0[1-9]|1[012])\\-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$", param)
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

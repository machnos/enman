package prices

import (
	"enman/internal"
	"enman/internal/http/api"
	"enman/internal/log"
	"enman/internal/persistency"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"net/http"
	"time"
)

const (
	errorCodePricesRoot             = "-prices"
	errorCodeStartDateParseError    = errorCodePricesRoot + "-01"
	errorCodeEndDateParseError      = errorCodePricesRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodePricesRoot + "-03"
	errorCodeUnableToLoadPrices     = errorCodePricesRoot + "-04"
	errorCodeUnableToLoadProviders  = errorCodePricesRoot + "-05"
)

type PricesApi struct {
	*api.BaseApi
}

func NewPricesApi(system *internal.System, repository persistency.Repository) *PricesApi {
	return &PricesApi{
		api.NewBaseApi(system, repository),
	}
}
func (p *PricesApi) prices(w http.ResponseWriter, r *http.Request) {
	type pricesResponsePrice struct {
		Time  time.Time `json:"time"`
		Price float32   `json:"price"`
	}
	type pricesResponse struct {
		Prices map[string][]pricesResponsePrice `json:"prices"`
	}
	rsp := pricesResponse{}
	var truncatedTo time.Duration
	startTime, _, err := p.ParseTimeFromRequestURL(r, "start", p.System.Location())
	if err != nil {
		log.Error(err.Error())
		p.ApiError(w, r, http.StatusBadRequest, errorCodeStartDateParseError, "Unable to parse start date")
		return
	}
	var endTime = time.Time{}
	if chi.URLParam(r, "end") != "" {
		var parsedTime time.Time
		parsedTime, truncatedTo, err = p.ParseTimeFromRequestURL(r, "end", p.System.Location())
		if err != nil {
			log.Error(err.Error())
			p.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateParseError, "Unable to parse end date")
			return
		}
		endTime = parsedTime
		if parsedTime.Before(startTime) {
			p.ApiError(w, r, http.StatusBadRequest, errorCodeEndDateBeforeStartDate, "End date is before start date")
			return
		}
	}
	if endTime.Equal(time.Time{}) {
		now := time.Now()
		endTime = time.Date(now.Year(), now.Month(), now.Day()+1, 23, 59, 59, 999999999, p.System.Location())
		truncatedTo = time.Hour
	}
	endTime = p.TruncateToEnd(endTime, truncatedTo)
	energyPrices, err := p.Repository.EnergyPrices(&startTime, &endTime, chi.URLParam(r, "provider"))
	if err != nil {
		log.Error(err.Error())
		p.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadPrices, err.Error())
		return
	}
	rsp.Prices = make(map[string][]pricesResponsePrice)
	for _, energyPrice := range energyPrices {
		rsp.Prices[energyPrice.Provider] = append(rsp.Prices[energyPrice.Provider], pricesResponsePrice{Time: energyPrice.Time, Price: energyPrice.Price})
	}
	render.JSON(w, r, rsp)
}

func (p *PricesApi) providers(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Providers []string `json:"providers"`
	}{}
	providers, err := p.Repository.EnergyPriceProviders()
	if err != nil {
		log.Error(err.Error())
		p.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadProviders, err.Error())
		return
	}
	rsp.Providers = providers
	render.JSON(w, r, rsp)
}

func (p *PricesApi) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/{start:%s}", p.TimePattern), p.prices)
		r.Get(fmt.Sprintf("/{start:%s}/{end:%s}", p.TimePattern, p.TimePattern), p.prices)
		r.Get("/providers", p.providers)
		r.Get(fmt.Sprintf("/{provider}/{start:%s}", p.TimePattern), p.prices)
		r.Get(fmt.Sprintf("/{provider}/{start:%s}/{end:%s}", p.TimePattern, p.TimePattern), p.prices)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

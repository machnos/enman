package prices

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

func NewPricesApi(system *domain.System, repository domain.Repository) *PricesApi {
	return &PricesApi{
		api.NewBaseApi(system, repository),
	}
}
func (p *PricesApi) prices(w http.ResponseWriter, r *http.Request) {
	type pricesResponsePrice struct {
		Time             time.Time `json:"time"`
		ConsumptionPrice float32   `json:"consumption_price"`
		FeedbackPrice    float32   `json:"feedback_price"`
	}
	type pricesResponse struct {
		Prices map[string][]pricesResponsePrice `json:"prices"`
	}
	rsp := pricesResponse{}

	startTime, endTime, success := p.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	energyPrices, err := p.Repository.EnergyPrices(startTime, endTime, chi.URLParam(r, "providerName"))
	if err != nil {
		log.Error(err.Error())
		p.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadPrices, err.Error())
		return
	}
	rsp.Prices = make(map[string][]pricesResponsePrice)
	for _, energyPrice := range energyPrices {
		rsp.Prices[energyPrice.Provider] = append(rsp.Prices[energyPrice.Provider], pricesResponsePrice{Time: energyPrice.Time, ConsumptionPrice: energyPrice.ConsumptionPrice, FeedbackPrice: energyPrice.FeedbackPrice})
	}
	render.JSON(w, r, rsp)
}

func (p *PricesApi) providers(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Providers []string `json:"providers"`
	}{}
	startTime, endTime, success := p.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	providers, err := p.Repository.EnergyPriceProviderNames(startTime, endTime)
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
		r.Get(fmt.Sprintf("/providers/{start:%s}", p.TimePattern), p.providers)
		r.Get(fmt.Sprintf("/providers/{start:%s}/{end:%s}", p.TimePattern, p.TimePattern), p.providers)
		r.Get(fmt.Sprintf("/{providerName}/{start:%s}", p.TimePattern), p.prices)
		r.Get(fmt.Sprintf("/{providerName}/{start:%s}/{end:%s}", p.TimePattern, p.TimePattern), p.prices)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

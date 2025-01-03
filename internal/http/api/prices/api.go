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

type Api struct {
	*api.BaseApi
}

func NewApi(system *domain.System, repository domain.Repository) *Api {
	return &Api{
		api.NewBaseApi(system, repository),
	}
}
func (api *Api) prices(w http.ResponseWriter, r *http.Request) {
	type pricesResponsePrice struct {
		Time             time.Time `json:"time"`
		ConsumptionPrice float32   `json:"consumption_price"`
		FeedbackPrice    float32   `json:"feedback_price"`
	}
	type pricesResponse struct {
		Prices map[string][]pricesResponsePrice `json:"prices"`
	}
	rsp := pricesResponse{}

	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	energyPrices, err := api.Repository.EnergyPrices(startTime, endTime, chi.URLParam(r, "providerName"))
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadPrices, err.Error())
		return
	}
	rsp.Prices = make(map[string][]pricesResponsePrice)
	for _, energyPrice := range energyPrices {
		rsp.Prices[energyPrice.Provider] = append(rsp.Prices[energyPrice.Provider], pricesResponsePrice{Time: energyPrice.Time, ConsumptionPrice: energyPrice.ConsumptionPrice, FeedbackPrice: energyPrice.FeedbackPrice})
	}
	render.JSON(w, r, rsp)
}

func (api *Api) providers(w http.ResponseWriter, r *http.Request) {
	rsp := struct {
		Providers []string `json:"providers"`
		Grid      string   `json:"grid"`
	}{}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	providers, err := api.Repository.EnergyPriceProviderNames(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadProviders, err.Error())
		return
	}
	rsp.Providers = providers
	rsp.Grid = api.System.Grid().Name()
	render.JSON(w, r, rsp)
}

func (api *Api) Router(subRoutes map[string]func(r chi.Router)) func(r chi.Router) {
	return func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))
		r.Get(fmt.Sprintf("/{start:%s}", api.TimePattern), api.prices)
		r.Get(fmt.Sprintf("/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.prices)
		r.Get(fmt.Sprintf("/providers/{start:%s}", api.TimePattern), api.providers)
		r.Get(fmt.Sprintf("/providers/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.providers)
		r.Get(fmt.Sprintf("/{providerName}/{start:%s}", api.TimePattern), api.prices)
		r.Get(fmt.Sprintf("/{providerName}/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.prices)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

package prices

import (
	"enman/internal/domain"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
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
	errorCodePricesRoot             = "-prices"
	errorCodeStartDateParseError    = errorCodePricesRoot + "-01"
	errorCodeEndDateParseError      = errorCodePricesRoot + "-02"
	errorCodeEndDateBeforeStartDate = errorCodePricesRoot + "-03"
	errorCodeUnableToLoadPrices     = errorCodePricesRoot + "-04"
	errorCodeUnableToLoadProviders  = errorCodePricesRoot + "-07"
)

type Api struct {
	*api.BaseApi
	repository repository.EnergyPrice
}

func NewApi(system *domain.System, repository repository.EnergyPrice) *Api {
	return &Api{
		api.NewBaseApi(system),
		repository,
	}
}
func (api *Api) prices(w http.ResponseWriter, r *http.Request) {
	type pricesResponsePrice struct {
		Time             time.Time `json:"time"`
		EnergyType       string    `json:"energy_type"`
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
	energyType, _ := prices.ParseEnergyType(chi.URLParam(r, "energyType"))
	energyPrices, err := api.repository.EnergyPrices(startTime, endTime, chi.URLParam(r, "providerName"), energyType)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadPrices, err.Error())
		return
	}
	rsp.Prices = make(map[string][]pricesResponsePrice)
	for _, energyPrice := range energyPrices {
		rsp.Prices[energyPrice.ProviderName] = append(rsp.Prices[energyPrice.ProviderName], pricesResponsePrice{
			Time:             energyPrice.Time,
			EnergyType:       energyPrice.EnergyType.String(),
			ConsumptionPrice: energyPrice.ConsumptionPrice,
			FeedbackPrice:    energyPrice.FeedbackPrice,
		})
	}
	render.JSON(w, r, rsp)
}

func (api *Api) providers(w http.ResponseWriter, r *http.Request) {
	type provider struct {
		Name        string   `json:"name"`
		EnergyTypes []string `json:"energy_types"`
	}

	rsp := struct {
		Providers []*provider `json:"providers"`
		Grid      string      `json:"grid"`
	}{}
	startTime, endTime, success := api.ValidateStartAndEndParams(w, r, errorCodeStartDateParseError, errorCodeEndDateParseError, errorCodeEndDateBeforeStartDate)
	if !success {
		return
	}
	providerRecords, err := api.repository.EnergyPriceProviders(startTime, endTime)
	if err != nil {
		log.Error(err.Error())
		api.ApiError(w, r, http.StatusInternalServerError, errorCodeUnableToLoadProviders, err.Error())
		return
	}
	for _, providerRecord := range providerRecords {
		rsp.Providers = append(rsp.Providers, &provider{
			Name:        providerRecord.Name,
			EnergyTypes: providerRecord.EnergyTypesAsStrings(),
		})
	}
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
		r.Get(fmt.Sprintf("/{providerName}/{energyType}/{start:%s}", api.TimePattern), api.prices)
		r.Get(fmt.Sprintf("/{providerName}/{energyType}/{start:%s}/{end:%s}", api.TimePattern, api.TimePattern), api.prices)
		if subRoutes != nil {
			for path, route := range subRoutes {
				r.Route(path, route)
			}
		}
	}
}

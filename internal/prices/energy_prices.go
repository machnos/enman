package prices

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

//https://api.energyzero.nl/v1/energyprices?fromDate=2023-01-19T23:00:00.000Z&tillDate=2023-01-20T22:59:59.999Z&interval=4&usageType=1&inclBtw=true

type EnergyPrice struct {
	Time     time.Time
	Price    float32
	Provider string
}

type PriceImporter interface {
	ImportPrices(startDate time.Time, endDate time.Time) error
}

type EnergyZeroPriceImporter struct {
	PriceImporter
}

func (e *EnergyZeroPriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://api.energyzero.nl/v1/energyprices?fromDate=%s&tillDate=%s&interval=4&usageType=1&inclBtw=true", startDate.UTC().Format(time.RFC3339), endDate.UTC().Format(time.RFC3339))
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)
	println(string(body))
	return nil
}

type EasyEnergyPriceImporter struct {
	PriceImporter
}

func (e *EasyEnergyPriceImporter) ImportPrices(startDate time.Time, endDate time.Time) error {
	if !startDate.Before(endDate) {
		return fmt.Errorf("start date must be before end date")
	}
	url := fmt.Sprintf("https://mijn.easyenergy.com/nl/api/tariff/getapxtariffs?startTimestamp=%s&endTimestamp=%s&includeVat=true", startDate.UTC().Format(time.RFC3339), endDate.UTC().Format(time.RFC3339))
	println(url)
	response, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)
	println(string(body))
	return nil
}

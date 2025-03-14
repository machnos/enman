package energyzero

import (
	"context"
	"encoding/json"
	"enman/internal/domain/prices"
	"enman/internal/log"
	"enman/internal/price_importers"
	"fmt"
	"io"
	"net/http"
	"time"
)

type PriceImporter struct {
	price_importers.PriceImporter
	*price_importers.BasePriceImporter
}

func (p *PriceImporter) ImportPrices(ctx context.Context, startDate time.Time, endDate time.Time) ([]*prices.EnergyPrice, error) {
	log.Info("Start reading energy prices from EnergyZero")
	if !startDate.Before(endDate) {
		return nil, fmt.Errorf("start date must be before end date")
	}
	result, err := p.importPricesForType(ctx, startDate, endDate, prices.EnergyTypeElectricity)
	if err != nil {
		return nil, err
	}
	gasPrices, err := p.importPricesForType(ctx, startDate, endDate, prices.EnergyTypeGas)
	result = append(result, gasPrices...)
	log.Info("Finished reading energy prices from EnergyZero")
	return result, nil
}

func (p *PriceImporter) importPricesForType(ctx context.Context, startDate time.Time, endDate time.Time, energyType prices.EnergyType) ([]*prices.EnergyPrice, error) {
	usageType := ""
	switch energyType {
	case prices.EnergyTypeElectricity:
		usageType = "1"
		break
	case prices.EnergyTypeGas:
		usageType = "3"
	default:
		return nil, fmt.Errorf("unsupported energy type: %s", energyType.String())
	}

	url := fmt.Sprintf("https://api.energyzero.nl/v1/energyprices?fromDate=%s&tillDate=%s&interval=4&usageType=%s&inclBtw=false", startDate.Truncate(time.Second).UTC().Format(time.RFC3339), endDate.Truncate(time.Second).UTC().Format(time.RFC3339), usageType)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, _ := io.ReadAll(response.Body)

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	result := make([]*prices.EnergyPrice, 0)
	_, hasPrices := data["prices"]
	if !hasPrices {
		return nil, fmt.Errorf("no prices found in energyzero response")
	}
	pricesData := data["Prices"].([]any)
	for _, object := range pricesData {
		pricePoint := object.(map[string]any)
		price := pricePoint["price"].(float64)
		pointStart, err := time.Parse(time.RFC3339, pricePoint["readingDate"].(string))
		if err != nil {
			return nil, err
		}
		energyPrice := &prices.EnergyPrice{
			Time:             pointStart.Truncate(time.Minute),
			ProviderName:     "EnergyZero",
			EnergyType:       energyType,
			ConsumptionPrice: float32(price),
			FeedbackPrice:    float32(price),
		}
		err = p.BasePriceImporter.Repository.StoreEnergyPrice(energyPrice)
		if err != nil {
			log.Warningf("failed to store energy price: %v", err)
		}
		result = append(result, energyPrice)
		go p.BasePriceImporter.FirePriceChangedEvent(ctx, energyPrice)
	}
	return result, nil
}

func NewEnergyZeroImporter(baseImporter *price_importers.BasePriceImporter) price_importers.PriceImporter {
	return &PriceImporter{
		BasePriceImporter: baseImporter,
	}
}

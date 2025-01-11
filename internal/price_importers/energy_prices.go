package price_importers

import (
	"context"
	"enman/internal/config"
	"enman/internal/domain/arithmetic"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type PriceImporter interface {
	ImportPrices(ctx context.Context, startDate time.Time, endDate time.Time) ([]*prices.EnergyPrice, error)
}

type BasePriceImporter struct {
	Repository       repository.EnergyPrice
	EnergyProviders  []*config.EnergyProvider
	registeredEvents sync.Map
}

func (b *BasePriceImporter) FirePriceChangedEvent(ctx context.Context, price *prices.EnergyPrice) {
	event := events.NewEnergyPriceValues().
		SetConsumptionPrice(price.ConsumptionPrice).
		SetFeedbackPrice(price.FeedbackPrice).
		SetEnergyProviderName(price.ProviderName).
		SetEnergyType(price.EnergyType).
		SetPriceStartingTime(price.Time)
	eventKey := fmt.Sprintf("%v-%v-%v", event.EnergyProviderName(), event.EnergyType().String(), event.PriceStartingTime())
	if time.Now().After(price.Time) {
		log.Tracef("Not registering price change task because it was in the past %s", eventKey)
		return
	}
	_, ok := b.registeredEvents.Load(eventKey)
	if ok {
		log.Tracef("Not registering price change task because it was already registered %s", eventKey)
		return
	}
	log.Tracef("Registering price change task %s", eventKey)
	b.registeredEvents.Store(eventKey, true)
	timer := time.NewTimer(time.Until(price.Time))
	defer timer.Stop()

	select {
	case <-timer.C:
		events.EnergyPrices.Trigger(event)
		b.registeredEvents.Delete(eventKey)
		log.Debugf("Deregistered price change task because it was fired %s", eventKey)
		return
	case <-ctx.Done():
		return
	}
}

func (b *BasePriceImporter) calculateProviderPrice(provider config.EnergyProvider, providerUpdateData *providerUpdateData) *prices.EnergyPrice {
	p := &prices.EnergyPrice{
		ProviderName: provider.Name,
		EnergyType:   providerUpdateData.energyType,
		Time:         providerUpdateData.time,
	}
	priceModels := make([]*config.PriceModel, 0)
	// Filter on PriceModels with the same EnergyType
	for _, priceModel := range provider.PriceModels {
		if priceModel.EnergyType == p.EnergyType.String() {
			priceModels = append(priceModels, priceModel)
		}
	}
	// Sort the remaining price models on start time.
	sort.Slice(priceModels, func(i, j int) bool {
		return priceModels[i].StartAsTime().Before(priceModels[j].StartAsTime())
	})

	ix := math.MinInt
	// set ix on the index of the last price model that is already valid.
	for i := 0; i < len(priceModels); i++ {
		if priceModels[i].StartAsTime().After(p.Time) {
			break
		}
		ix = i
	}
	if ix < 0 {
		return nil
	}
	// consumption price
	formula := priceModels[ix].ConsumptionFormula
	if formula != "" {
		value, err := arithmetic.ParseCalculation(formula, providerUpdateData.baseConsumptionPrices)
		if err != nil {
			log.Warningf("Unable to calculate consumption price: %v", err)
			return nil
		}
		p.ConsumptionPrice = float32(value)
	}
	// feedback price
	formula = priceModels[ix].FeedbackFormula
	if formula != "" {
		value, err := arithmetic.ParseCalculation(formula, providerUpdateData.baseFeedbackPrices)
		if err != nil {
			log.Warningf("Unable to calculate feedback price: %v", err)
			return nil
		}
		p.FeedbackPrice = float32(value)
	}
	return p
}

func (b *BasePriceImporter) UpdateProviderPrices(ctx context.Context, rootPrices []*prices.EnergyPrice) {
	updateData := map[time.Time]map[prices.EnergyType]*providerUpdateData{}
	for _, rootPrice := range rootPrices {
		startTime := rootPrice.Time.Truncate(time.Minute)
		timeData, ok := updateData[startTime]
		if !ok {
			timeData = map[prices.EnergyType]*providerUpdateData{}
			timeData[rootPrice.EnergyType] = &providerUpdateData{
				startTime,
				rootPrice.EnergyType,
				map[string]float64{
					rootPrice.ProviderName: float64(rootPrice.ConsumptionPrice),
				},
				map[string]float64{
					rootPrice.ProviderName: float64(rootPrice.FeedbackPrice),
				},
			}
			updateData[startTime] = timeData
		} else {
			data, ok := timeData[rootPrice.EnergyType]
			if !ok {
				timeData[rootPrice.EnergyType] = &providerUpdateData{
					startTime,
					rootPrice.EnergyType,
					map[string]float64{
						rootPrice.ProviderName: float64(rootPrice.ConsumptionPrice),
					},
					map[string]float64{
						rootPrice.ProviderName: float64(rootPrice.FeedbackPrice),
					},
				}
			} else {
				data.baseConsumptionPrices[rootPrice.ProviderName] = float64(rootPrice.ConsumptionPrice)
				data.baseFeedbackPrices[rootPrice.ProviderName] = float64(rootPrice.FeedbackPrice)
			}
		}
	}

	for ix := 0; ix < len(b.EnergyProviders); ix++ {
		for _, timeData := range updateData {
			for _, pud := range timeData {
				energyProviderPrice := b.calculateProviderPrice(*b.EnergyProviders[ix], pud)
				if energyProviderPrice != nil {
					err := b.Repository.StoreEnergyPrice(energyProviderPrice)
					if err != nil {
						log.Warningf("failed to store energy price: %v", err)
					}
					go b.FirePriceChangedEvent(ctx, energyProviderPrice)
				}
			}
		}
	}
}

type providerUpdateData struct {
	time                  time.Time
	energyType            prices.EnergyType
	baseConsumptionPrices map[string]float64
	baseFeedbackPrices    map[string]float64
}

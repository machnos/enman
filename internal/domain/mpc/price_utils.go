package mpc

import (
	"enman/internal/domain/prices"
	"time"
)

// BuildPriceForecast converts energy prices to a slice of PricePoints for the optimization horizon
func BuildPriceForecast(currentTime time.Time, energyPrices []*prices.EnergyPrice) []PricePoint {
	forecast := make([]PricePoint, 0, NumTimeSteps)

	currentHour := currentTime.Truncate(time.Hour)

	for i := 0; i < NumTimeSteps; i++ {
		targetTime := currentHour.Add(time.Duration(i) * time.Hour)
		targetEndTime := targetTime.Add(time.Hour)

		// Find the price that covers this hour
		var matchedPrice *prices.EnergyPrice
		for _, ep := range energyPrices {
			if (ep.Time.Equal(targetTime) || ep.Time.Before(targetTime)) &&
				ep.EndTime.After(targetTime) {
				matchedPrice = ep
				break
			}
		}

		if matchedPrice != nil {
			forecast = append(forecast, PricePoint{
				Time:             targetTime,
				EndTime:          targetEndTime,
				ConsumptionPrice: matchedPrice.ConsumptionPrice,
				FeedbackPrice:    matchedPrice.FeedbackPrice,
			})
		} else {
			// Use a default high price if no price data available
			// This encourages the system to not charge during unknown periods
			forecast = append(forecast, PricePoint{
				Time:             targetTime,
				EndTime:          targetEndTime,
				ConsumptionPrice: 0.30, // Default high price €/kWh
				FeedbackPrice:    0.05, // Default low feedback price €/kWh
			})
		}
	}

	return forecast
}

// FindCheapestHours returns the indices of the N cheapest hours in the price forecast
func FindCheapestHours(prices []PricePoint, count int) []int {
	if count <= 0 || len(prices) == 0 {
		return []int{}
	}

	// Create indexed slice
	type indexedPrice struct {
		index int
		price float32
	}

	indexed := make([]indexedPrice, len(prices))
	for i, p := range prices {
		indexed[i] = indexedPrice{i, p.ConsumptionPrice}
	}

	// Sort by price (simple bubble sort for small arrays)
	for i := 0; i < len(indexed)-1; i++ {
		for j := i + 1; j < len(indexed); j++ {
			if indexed[j].price < indexed[i].price {
				indexed[i], indexed[j] = indexed[j], indexed[i]
			}
		}
	}

	// Return indices of cheapest hours
	result := make([]int, min(count, len(indexed)))
	for i := 0; i < len(result); i++ {
		result[i] = indexed[i].index
	}

	return result
}

// FindMostExpensiveHours returns the indices of the N most expensive hours in the price forecast
func FindMostExpensiveHours(prices []PricePoint, count int) []int {
	if count <= 0 || len(prices) == 0 {
		return []int{}
	}

	// Create indexed slice
	type indexedPrice struct {
		index int
		price float32
	}

	indexed := make([]indexedPrice, len(prices))
	for i, p := range prices {
		indexed[i] = indexedPrice{i, p.ConsumptionPrice}
	}

	// Sort by price descending
	for i := 0; i < len(indexed)-1; i++ {
		for j := i + 1; j < len(indexed); j++ {
			if indexed[j].price > indexed[i].price {
				indexed[i], indexed[j] = indexed[j], indexed[i]
			}
		}
	}

	// Return indices of most expensive hours
	result := make([]int, min(count, len(indexed)))
	for i := 0; i < len(result); i++ {
		result[i] = indexed[i].index
	}

	return result
}

// CalculatePriceSpread returns the difference between max and min prices
func CalculatePriceSpread(prices []PricePoint) float32 {
	if len(prices) == 0 {
		return 0
	}

	minPrice := prices[0].ConsumptionPrice
	maxPrice := prices[0].ConsumptionPrice

	for _, p := range prices {
		if p.ConsumptionPrice < minPrice {
			minPrice = p.ConsumptionPrice
		}
		if p.ConsumptionPrice > maxPrice {
			maxPrice = p.ConsumptionPrice
		}
	}

	return maxPrice - minPrice
}

// CalculateAveragePrice returns the average consumption price
func CalculateAveragePrice(prices []PricePoint) float32 {
	if len(prices) == 0 {
		return 0
	}

	var total float32
	for _, p := range prices {
		total += p.ConsumptionPrice
	}

	return total / float32(len(prices))
}

// IsPriceAboveAverage returns true if the price at the given index is above average
func IsPriceAboveAverage(prices []PricePoint, index int) bool {
	if index < 0 || index >= len(prices) {
		return false
	}

	avg := CalculateAveragePrice(prices)
	return prices[index].ConsumptionPrice > avg
}

// CalculatePotentialArbitrage calculates the potential profit from charging at cheap hours
// and discharging at expensive hours, considering battery efficiency
func CalculatePotentialArbitrage(prices []PricePoint, efficiency float32, capacityKWh float32) float32 {
	if len(prices) < 2 || capacityKWh <= 0 {
		return 0
	}

	// Find cheapest and most expensive prices
	cheapest := prices[0].ConsumptionPrice
	mostExpensive := prices[0].ConsumptionPrice

	for _, p := range prices {
		if p.ConsumptionPrice < cheapest {
			cheapest = p.ConsumptionPrice
		}
		if p.ConsumptionPrice > mostExpensive {
			mostExpensive = p.ConsumptionPrice
		}
	}

	// Calculate potential arbitrage profit
	// Buy at cheap price, sell at expensive price, accounting for efficiency loss
	effFactor := efficiency / 100.0
	costToBuy := cheapest * capacityKWh
	revenueFromSell := mostExpensive * capacityKWh * effFactor

	return revenueFromSell - costToBuy
}

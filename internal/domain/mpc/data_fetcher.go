package mpc

import (
	"enman/internal/domain/constants"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"time"
)

// DataFetcher handles fetching and processing historical data for MPC optimization
type DataFetcher struct {
	repository repository.Repository
}

// NewDataFetcher creates a new DataFetcher instance
func NewDataFetcher(repo repository.Repository) *DataFetcher {
	return &DataFetcher{
		repository: repo,
	}
}

// FetchHistoricalGridConsumption fetches historical grid consumption data
// and calculates hourly averages for forecasting
func (df *DataFetcher) FetchHistoricalGridConsumption(gridName string, daysBack int) (*GridConsumptionData, error) {
	result := &GridConsumptionData{
		LastUpdated: time.Now(),
	}

	if df.repository == nil {
		return result, nil
	}

	now := time.Now()
	from := now.AddDate(0, 0, -daysBack)

	// Fetch hourly aggregated electricity usage data with min and max
	// The difference between max and min meter readings gives us the energy consumed in that hour
	aggregateConfig := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionMin, repository.AggregateFunctionMax},
		CreateEmpty:  false,
	}

	records, err := df.repository.ElectricityUsages(from, now, gridName, aggregateConfig)
	if err != nil {
		log.Debugf("Failed to fetch historical grid consumption: %v", err)
		return result, err
	}

	// Calculate hourly averages
	hourlyTotals := make([]float64, 24)
	hourlyCounts := make([]int, 24)
	dayOfWeekTotals := make([]float64, 7)
	dayOfWeekCounts := make([]int, 7)

	for _, record := range records {
		if record.Role != string(constants.EnergySourceRoleGrid) {
			continue
		}

		hour := record.StartTime.Hour()
		dayOfWeek := int(record.StartTime.Weekday())

		minUsage, hasMin := record.Usages[repository.AggregateFunctionMin]
		maxUsage, hasMax := record.Usages[repository.AggregateFunctionMax]

		if hasMin && hasMax {
			// Calculate energy consumed in this hour as the difference between max and min readings
			// This gives us kWh consumed in that hour
			energyConsumedKWh := maxUsage.TotalEnergyConsumed() - minUsage.TotalEnergyConsumed()

			// Convert kWh to average power in Watts (kWh per hour = kW, * 1000 = W)
			// Since this is for a 1-hour window, kWh directly equals average kW
			powerWatts := energyConsumedKWh * 1000

			// Only count positive consumption
			if powerWatts > 0 {
				hourlyTotals[hour] += powerWatts
				hourlyCounts[hour]++

				dayOfWeekTotals[dayOfWeek] += powerWatts
				dayOfWeekCounts[dayOfWeek]++

				result.DataPointCount++
			}
		}
	}

	// Calculate averages
	var overallAvg float64
	for i := 0; i < 24; i++ {
		if hourlyCounts[i] > 0 {
			result.HourlyAverages[i] = float32(hourlyTotals[i] / float64(hourlyCounts[i]))
			overallAvg += float64(result.HourlyAverages[i])
		}
	}
	overallAvg /= 24

	// Calculate day of week factors (relative to average)
	for i := 0; i < 7; i++ {
		if dayOfWeekCounts[i] > 0 && overallAvg > 0 {
			result.DayOfWeekFactors[i] = float32(dayOfWeekTotals[i] / float64(dayOfWeekCounts[i]) / overallAvg)
		} else {
			result.DayOfWeekFactors[i] = 1.0
		}
	}

	// Calculate weekend factor
	weekdaySum := float32(0)
	weekendSum := float32(0)
	for i := 0; i < 7; i++ {
		if i == 0 || i == 6 { // Sunday or Saturday
			weekendSum += result.DayOfWeekFactors[i]
		} else {
			weekdaySum += result.DayOfWeekFactors[i]
		}
	}
	if weekdaySum > 0 {
		result.WeekendFactor = (weekendSum / 2) / (weekdaySum / 5)
	} else {
		result.WeekendFactor = 1.0
	}

	return result, nil
}

// FetchHistoricalBatteryConsumption fetches historical battery discharge data
func (df *DataFetcher) FetchHistoricalBatteryConsumption(batteryNames []string, daysBack int) (*HistoricalData, error) {
	result := &HistoricalData{
		LastUpdated:    time.Now(),
		SeasonalFactor: 1.0,
	}

	if df.repository == nil || len(batteryNames) == 0 {
		return result, nil
	}

	now := time.Now()
	from := now.AddDate(0, 0, -daysBack)

	aggregateConfig := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionMean},
		CreateEmpty:  false,
	}

	hourlyTotals := make([]float64, 24)
	hourlyCounts := make([]int, 24)

	for _, batteryName := range batteryNames {
		records, err := df.repository.BatteryStates(from, now, batteryName, aggregateConfig)
		if err != nil {
			log.Debugf("Failed to fetch battery states for %s: %v", batteryName, err)
			continue
		}

		for _, record := range records {
			hour := record.StartTime.Hour()

			if meanState, ok := record.States[repository.AggregateFunctionMean]; ok {
				// Use power (negative means discharging)
				power := meanState.Power()
				if power < 0 {
					// Only count discharge power
					hourlyTotals[hour] += float64(-power)
					hourlyCounts[hour]++
					result.DataPointCount++
				}
			}
		}
	}

	// Calculate averages
	for i := 0; i < 24; i++ {
		if hourlyCounts[i] > 0 {
			result.HourlyAverages[i] = float32(hourlyTotals[i] / float64(hourlyCounts[i]))
		}
	}

	return result, nil
}

// FetchHistoricalPVProduction fetches historical PV production data
func (df *DataFetcher) FetchHistoricalPVProduction(pvNames []string, daysBack int) (*PVProductionData, error) {
	result := &PVProductionData{
		LastUpdated: time.Now(),
	}

	// Initialize monthly factors with reasonable defaults
	// Northern hemisphere seasonal pattern
	result.MonthlyFactors = [12]float32{
		0.3, 0.4, 0.6, 0.8, 1.0, 1.1, // Jan-Jun
		1.1, 1.0, 0.8, 0.6, 0.4, 0.3, // Jul-Dec
	}

	if df.repository == nil {
		// Return typical solar production pattern if no data
		result.HourlyAverages = [24]float32{
			0, 0, 0, 0, 0, 0, // 00:00-05:00
			0, 100, 500, 1000, 1500, 2000, // 06:00-11:00
			2200, 2000, 1500, 1000, 500, 100, // 12:00-17:00
			0, 0, 0, 0, 0, 0, // 18:00-23:00
		}
		return result, nil
	}

	now := time.Now()
	from := now.AddDate(0, 0, -daysBack)

	aggregateConfig := &repository.AggregateConfiguration{
		WindowUnit:   repository.WindowUnitHour,
		WindowAmount: 1,
		Functions:    []repository.AggregateFunction{repository.AggregateFunctionMin, repository.AggregateFunctionMax},
		CreateEmpty:  false,
	}

	hourlyTotals := make([]float64, 24)
	hourlyCounts := make([]int, 24)
	monthlyTotals := make([]float64, 12)
	monthlyCounts := make([]int, 12)

	// Fetch source names if none provided
	if len(pvNames) == 0 {
		names, err := df.repository.ElectricitySourceNames(from, now)
		if err == nil {
			for _, name := range names {
				// Try to identify PV sources by name pattern
				pvNames = append(pvNames, name)
			}
		}
	}

	for _, pvName := range pvNames {
		records, err := df.repository.ElectricityUsages(from, now, pvName, aggregateConfig)
		if err != nil {
			log.Debugf("Failed to fetch PV production for %s: %v", pvName, err)
			continue
		}

		for _, record := range records {
			// Only count PV role sources
			if record.Role != string(constants.EnergySourceRolePv) {
				continue
			}

			hour := record.StartTime.Hour()
			month := int(record.StartTime.Month()) - 1

			minUsage, hasMin := record.Usages[repository.AggregateFunctionMin]
			maxUsage, hasMax := record.Usages[repository.AggregateFunctionMax]

			if hasMin && hasMax {
				// Calculate energy produced in this hour as the difference between max and min readings
				// PV production is recorded as "energy provided"
				energyProducedKWh := maxUsage.TotalEnergyProvided() - minUsage.TotalEnergyProvided()

				// Convert kWh to average power in Watts
				powerWatts := energyProducedKWh * 1000

				if powerWatts > 0 {
					hourlyTotals[hour] += powerWatts
					hourlyCounts[hour]++

					monthlyTotals[month] += powerWatts
					monthlyCounts[month]++

					result.DataPointCount++
				}
			}
		}
	}

	// Calculate hourly averages
	for i := 0; i < 24; i++ {
		if hourlyCounts[i] > 0 {
			result.HourlyAverages[i] = float32(hourlyTotals[i] / float64(hourlyCounts[i]))
		}
	}

	// Calculate monthly factors (relative to average)
	var avgMonthly float64
	for i := 0; i < 12; i++ {
		if monthlyCounts[i] > 0 {
			avgMonthly += monthlyTotals[i] / float64(monthlyCounts[i])
		}
	}
	avgMonthly /= 12

	if avgMonthly > 0 {
		for i := 0; i < 12; i++ {
			if monthlyCounts[i] > 0 {
				result.MonthlyFactors[i] = float32(monthlyTotals[i] / float64(monthlyCounts[i]) / avgMonthly)
			}
		}
	}

	return result, nil
}

// GetSeasonalFactor returns a seasonal adjustment factor based on the current date
func GetSeasonalFactor(t time.Time) float32 {
	month := t.Month()

	// Northern hemisphere pattern
	// Summer (high solar, low heating demand)
	// Winter (low solar, high heating demand)
	switch month {
	case time.December, time.January, time.February:
		return 1.3 // Higher consumption in winter
	case time.March, time.April, time.May:
		return 1.0 // Average in spring
	case time.June, time.July, time.August:
		return 0.8 // Lower consumption in summer
	case time.September, time.October, time.November:
		return 1.0 // Average in autumn
	default:
		return 1.0
	}
}

// GetDayTypeMultiplier returns a multiplier based on whether it's a weekday or weekend
func GetDayTypeMultiplier(t time.Time) float32 {
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return 0.9 // Slightly lower consumption on weekends typically
	}
	return 1.0
}

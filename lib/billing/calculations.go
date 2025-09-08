package billing

import (
	"electricity-invoice-calculator/lib/eloverblik"
	"electricity-invoice-calculator/lib/energinet"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// HourlyTariffCost represents the cost breakdown for a single hour
type HourlyTariffCost struct {
	DateTime     time.Time
	Consumption  float64
	TariffCosts  map[string]float64 // tariff name -> cost in DKK
	SupplierCost float64            // electricity supplier cost in DKK
	SpotPrice    float64            // spot price DKK/kWh
	SpotCost     float64            // spot price cost in DKK
	TotalCost    float64            // total of all tariffs + supplier cost + spot cost
}

// GridCompany represents a grid company mapping
type GridCompany struct {
	Def       string `json:"def"`
	Name      string `json:"name"`
	PriceArea string `json:"priceArea"`
}

// GridCompaniesMapping represents the JSON structure
type GridCompaniesMapping struct {
	GridCompanies []GridCompany `json:"gridCompanies"`
}

// LoadGridCompaniesMapping loads the grid companies mapping from JSON file
func LoadGridCompaniesMapping(filename string) (*GridCompaniesMapping, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %v", filename, err)
	}
	defer file.Close()

	var mapping GridCompaniesMapping
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&mapping)
	if err != nil {
		return nil, fmt.Errorf("could not parse JSON in %s: %v", filename, err)
	}

	return &mapping, nil
}

// FindPriceArea finds the price area (DK1/DK2) for a given grid area identification
func FindPriceArea(gridAreaName string, mapping *GridCompaniesMapping) (string, error) {
	for _, company := range mapping.GridCompanies {
		if company.Name == gridAreaName {
			return company.PriceArea, nil
		}
	}
	return "", fmt.Errorf("grid area ID %s not found in mapping", gridAreaName)
}

// GetSpotPriceForHour gets the spot price for a specific hour from spot price data
func GetSpotPriceForHour(hourDateTime time.Time, spotPrices []energinet.SpotPriceRecord) (float64, error) {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	localTime := hourDateTime.In(copenhagen)

	// Format to match the HourDK format in spot price data (YYYY-MM-DDTHH:00:00)
	hourDK := localTime.Format("2006-01-02T15:04:05")

	for _, record := range spotPrices {
		if record.HourDK == hourDK {
			// Convert from DKK/MWh to DKK/kWh
			return energinet.ConvertToKWh(record.SpotPriceDKK), nil
		}
	}

	return 0, fmt.Errorf("no spot price found for hour %s", hourDK)
}

// CalculateHourlyTariffs calculates all tariff costs for a single hour of consumption
// supplierPricePerKWh is the electricity supplier's price in DKK per kWh (e.g., 0.02 for 2 øre)
// spotPrices contains the spot price data for the period
func CalculateHourlyTariffs(hourlyConsumption eloverblik.HourlyConsumption, chargesData *eloverblik.ChargesResult, supplierPricePerKWh float64, spotPrices []energinet.SpotPriceRecord) HourlyTariffCost {
	// Convert to Copenhagen time and get the hour position (1-24)
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	localTime := hourlyConsumption.DateTime.In(copenhagen)
	hourPosition := localTime.Hour() + 1 // Convert 0-23 to 1-24 for tariff positions

	// Initialize result
	result := HourlyTariffCost{
		DateTime:     hourlyConsumption.DateTime,
		Consumption:  hourlyConsumption.Consumption,
		TariffCosts:  make(map[string]float64),
		SupplierCost: 0.0,
		SpotPrice:    0.0,
		SpotCost:     0.0,
		TotalCost:    0.0,
	}

	// Get spot price for this hour
	spotPrice, err := GetSpotPriceForHour(hourlyConsumption.DateTime, spotPrices)
	if err != nil {
		// If we can't find spot price, log but continue (maybe set to 0 or handle differently)
		fmt.Printf("Warning: %v\n", err)
		spotPrice = 0.0
	}
	result.SpotPrice = spotPrice
	result.SpotCost = hourlyConsumption.Consumption * spotPrice

	// Calculate cost for each tariff
	var totalTariffCost float64
	for _, tariff := range chargesData.Tariffs {
		var applicablePrice float64

		if tariff.PeriodType == "P1D" && len(tariff.Prices) == 1 {
			// Daily fixed tariff - same price all day (transmission, system, elafgift)
			applicablePrice = tariff.Prices[0].Price
		} else if tariff.PeriodType == "PT1H" && len(tariff.Prices) == 24 {
			// Hourly variable tariff (Nettarif C) - different price per hour
			for _, price := range tariff.Prices {
				if price.Position == fmt.Sprintf("%d", hourPosition) {
					applicablePrice = price.Price
					break
				}
			}
		} else {
			// Fallback for unexpected tariff structures
			if len(tariff.Prices) == 1 {
				applicablePrice = tariff.Prices[0].Price
			} else {
				// Try to find matching position
				for _, price := range tariff.Prices {
					if price.Position == fmt.Sprintf("%d", hourPosition) {
						applicablePrice = price.Price
						break
					}
				}
			}
		}

		// Calculate cost for this tariff
		hourlyTariffCost := hourlyConsumption.Consumption * applicablePrice
		result.TariffCosts[tariff.Name] = hourlyTariffCost
		totalTariffCost += hourlyTariffCost
	}

	// Calculate supplier cost
	result.SupplierCost = hourlyConsumption.Consumption * supplierPricePerKWh

	// Calculate total cost (all tariffs + supplier cost + spot cost)
	result.TotalCost = totalTariffCost + result.SupplierCost + result.SpotCost

	return result
}

// CalculateAllHourlyTariffs calculates tariff costs for all hours in the consumption data
func CalculateAllHourlyTariffs(consumptionData []eloverblik.HourlyConsumption, chargesData *eloverblik.ChargesResult, supplierPricePerKWh float64, spotPrices []energinet.SpotPriceRecord) []HourlyTariffCost {
	var results []HourlyTariffCost

	for _, hourlyConsumption := range consumptionData {
		hourlyCost := CalculateHourlyTariffs(hourlyConsumption, chargesData, supplierPricePerKWh, spotPrices)
		results = append(results, hourlyCost)
	}

	return results
}

// SummarizeTariffCosts creates a summary of all tariff costs across all hours
func SummarizeTariffCosts(hourlyTariffCosts []HourlyTariffCost) map[string]float64 {
	summary := make(map[string]float64)

	for _, hourlyCost := range hourlyTariffCosts {
		for tariffName, cost := range hourlyCost.TariffCosts {
			summary[tariffName] += cost
		}
	}

	return summary
}

// GetTotalTariffCosts calculates the total cost across all tariffs and all hours
func GetTotalTariffCosts(hourlyTariffCosts []HourlyTariffCost) float64 {
	var total float64

	for _, hourlyCost := range hourlyTariffCosts {
		total += hourlyCost.TotalCost
	}

	return total
}

// GetTotalSupplierCosts calculates the total supplier cost across all hours
func GetTotalSupplierCosts(hourlyTariffCosts []HourlyTariffCost) float64 {
	var total float64

	for _, hourlyCost := range hourlyTariffCosts {
		total += hourlyCost.SupplierCost
	}

	return total
}

// GetTotalSpotCosts calculates the total spot cost across all hours
func GetTotalSpotCosts(hourlyTariffCosts []HourlyTariffCost) float64 {
	var total float64

	for _, hourlyCost := range hourlyTariffCosts {
		total += hourlyCost.SpotCost
	}

	return total
}

// FetchSpotPricesForPeriod fetches spot prices for the given period and price area
func FetchSpotPricesForPeriod(startDate, endDate time.Time, priceArea string) ([]energinet.SpotPriceRecord, error) {
	// Format dates for the API
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Get spot prices from Energinet API
	spotPrices, err := energinet.GetSpotPrices(startDateStr, endDateStr, []string{priceArea})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spot prices: %v", err)
	}

	return spotPrices, nil
}

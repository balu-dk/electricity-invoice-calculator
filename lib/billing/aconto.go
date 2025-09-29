package billing

import (
	"electricity-invoice-calculator/lib/eloverblik"
	"electricity-invoice-calculator/lib/energinet"
	"electricity-invoice-calculator/lib/utils"
	"fmt"
	"time"
)

// PeriodType definerer om perioden er historisk, aconto eller hybrid
type PeriodType string

const (
	PeriodHistorical PeriodType = "historical" // Kun tidligere data
	PeriodAconto     PeriodType = "aconto"     // Kun estimeret data
	PeriodHybrid     PeriodType = "hybrid"     // Faktisk + estimeret data
)

// AcontoEstimation contains estimated data for aconto calculations
type AcontoEstimation struct {
	EstimatedConsumption []eloverblik.HourlyConsumption
	EstimatedSpotPrices  []energinet.SpotPriceRecord
	EstimationMethod     string
	TotalEstimatedkWh    float64
	AvgHourlyConsumption float64
	AvgSpotPrice         float64
	HoursInPeriod        int
}

// HybridEstimation indeholder både faktiske og estimerede data
type HybridEstimation struct {
	// Faktiske data (allerede afregnet)
	ActualConsumption []eloverblik.HourlyConsumption
	ActualSpotPrices  []energinet.SpotPriceRecord
	ActualTotalKWh    float64

	// Estimerede data (resterende periode)
	EstimatedConsumption []eloverblik.HourlyConsumption
	EstimatedSpotPrices  []energinet.SpotPriceRecord
	EstimatedTotalKWh    float64

	// Kombinerede data (til videre beregninger)
	CombinedConsumption []eloverblik.HourlyConsumption
	CombinedSpotPrices  []energinet.SpotPriceRecord
	TotalEstimatedkWh   float64

	// Metadata
	SplitDateTime    time.Time // Hvor delingen sker (nu-tidspunkt)
	EstimationMethod string    // Beskrivelse af metode
	ActualHours      int       // Antal faktiske timer
	EstimatedHours   int       // Antal estimerede timer
	FixedSpotPrice   float64   // Fast pris for estimerede timer
}

// DeterminePeriodType finder ud af hvilken type periode vi har
func DeterminePeriodType(period Period, calculationType CalculationType) PeriodType {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	now := time.Now().In(copenhagen)

	if calculationType == Historical {
		return PeriodHistorical
	}

	// Hvis perioden starter i fremtiden = ren aconto
	if period.Start.After(now) {
		return PeriodAconto
	}

	// Hvis perioden er helt afsluttet = historisk (selvom brugeren valgte aconto)
	if period.End.Before(now) || period.End.Equal(now) {
		return PeriodHistorical
	}

	// Hvis vi er inde i perioden = hybrid
	if period.Start.Before(now) && period.End.After(now) {
		return PeriodHybrid
	}

	return PeriodAconto
}

// EstimateConsumptionForPeriod creates estimated hourly consumption data
// based on EstimatedAnnualVolume using industry-standard monthly division
func EstimateConsumptionForPeriod(estimatedAnnualVolume int, startDate, endDate time.Time, frequency BillingFrequency) ([]eloverblik.HourlyConsumption, error) {
	// Calculate hours in the period
	duration := endDate.Sub(startDate)
	hoursInPeriod := int(duration.Hours())

	// Calculate total consumption for this period using industry-standard approach
	var totalPeriodConsumption float64

	if frequency == Monthly {
		// For monthly: use 0.0833 factor (matches el leverandør practice)
		totalPeriodConsumption = float64(estimatedAnnualVolume) * 0.0833
	} else {
		// For quarterly: use 0.25 factor (1/4)
		totalPeriodConsumption = float64(estimatedAnnualVolume) * 0.25
	}

	// Calculate average hourly consumption (simple even distribution within period)
	avgHourlyConsumption := totalPeriodConsumption / float64(hoursInPeriod)

	// Create hourly consumption data
	var estimatedConsumption []eloverblik.HourlyConsumption

	currentTime := startDate
	for currentTime.Before(endDate) {
		hourlyData := eloverblik.HourlyConsumption{
			DateTime:    currentTime,
			Consumption: avgHourlyConsumption,
			Quality:     "ESTIMATED", // Mark as estimated data
		}

		estimatedConsumption = append(estimatedConsumption, hourlyData)
		currentTime = currentTime.Add(1 * time.Hour)
	}

	return estimatedConsumption, nil
}

// EstimateSpotPricesForPeriod creates estimated spot prices for the aconto period
// using a fixed estimate based on reasonable market averages
func EstimateSpotPricesForPeriod(startDate, endDate time.Time, priceArea string) ([]energinet.SpotPriceRecord, error) {
	// Use a simple fixed spot price estimate in DKK/kWh
	estimatedSpotPriceDKKPerKWh := 0.614029 // 0.61 DKK/kWh as a reasonable estimate

	// Convert to MWh for the SpotPriceRecord (since that's what the struct expects)
	estimatedSpotPriceDKKPerMWh := estimatedSpotPriceDKKPerKWh * 1000.0 // Convert kWh to MWh

	var estimatedSpotPrices []energinet.SpotPriceRecord

	// Create spot price records for each hour in the period
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	currentTime := startDate

	for currentTime.Before(endDate) {
		// Format times for the spot price record
		hourUTC := currentTime.UTC().Format("2006-01-02T15:04:05")
		hourDK := currentTime.In(copenhagen).Format("2006-01-02T15:04:05")

		spotRecord := energinet.SpotPriceRecord{
			HourUTC:      hourUTC,
			HourDK:       hourDK,
			PriceArea:    priceArea,
			SpotPriceDKK: estimatedSpotPriceDKKPerMWh,
			SpotPriceEUR: estimatedSpotPriceDKKPerMWh / 7.45, // Rough EUR conversion
		}

		estimatedSpotPrices = append(estimatedSpotPrices, spotRecord)
		currentTime = currentTime.Add(1 * time.Hour)
	}

	return estimatedSpotPrices, nil
}

// EstimateHistoricalSpotPricesForPeriod gets actual spot prices from the same period last year
func EstimateHistoricalSpotPricesForPeriod(startDate, endDate time.Time, priceArea string) ([]energinet.SpotPriceRecord, error) {
	// Get same period from previous year
	lastYearStart := startDate.AddDate(-1, 0, 0)
	lastYearEnd := endDate.AddDate(-1, 0, 0)

	// Fetch historical spot prices
	historicalSpotPrices, err := FetchSpotPricesForPeriod(lastYearStart, lastYearEnd, priceArea)
	if err != nil {
		// Return error, let caller decide fallback strategy
		return nil, fmt.Errorf("could not fetch historical spot prices: %v", err)
	}

	// Adjust the dates to current year but keep the prices
	var adjustedSpotPrices []energinet.SpotPriceRecord
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")

	currentTime := startDate
	historicalIndex := 0

	for currentTime.Before(endDate) && historicalIndex < len(historicalSpotPrices) {
		// Format times for current year
		hourUTC := currentTime.UTC().Format("2006-01-02T15:04:05")
		hourDK := currentTime.In(copenhagen).Format("2006-01-02T15:04:05")

		// Use historical price but with current dates
		adjustedRecord := energinet.SpotPriceRecord{
			HourUTC:      hourUTC,
			HourDK:       hourDK,
			PriceArea:    priceArea,
			SpotPriceDKK: historicalSpotPrices[historicalIndex].SpotPriceDKK,
			SpotPriceEUR: historicalSpotPrices[historicalIndex].SpotPriceEUR,
		}

		adjustedSpotPrices = append(adjustedSpotPrices, adjustedRecord)
		currentTime = currentTime.Add(1 * time.Hour)
		historicalIndex++
	}

	if len(adjustedSpotPrices) == 0 {
		return nil, fmt.Errorf("insufficient historical data available")
	}

	return adjustedSpotPrices, nil
}

// CreateAcontoEstimation creates a complete estimation for aconto calculation
func CreateAcontoEstimation(estimatedAnnualVolume int, startDate, endDate time.Time, priceArea string, frequency BillingFrequency, useHistoricalPrices bool) (*AcontoEstimation, error) {
	// Estimate consumption
	estimatedConsumption, err := EstimateConsumptionForPeriod(estimatedAnnualVolume, startDate, endDate, frequency)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate consumption: %v", err)
	}

	// Estimate spot prices
	var estimatedSpotPrices []energinet.SpotPriceRecord
	var estimationMethod string

	if useHistoricalPrices {
		estimatedSpotPrices, err = EstimateHistoricalSpotPricesForPeriod(startDate, endDate, priceArea)
		if err != nil {
			// Fallback to fixed estimate if historical data fails
			estimatedSpotPrices, err = EstimateSpotPricesForPeriod(startDate, endDate, priceArea)
			if err != nil {
				return nil, fmt.Errorf("failed to estimate spot prices: %v", err)
			}
			estimationMethod = "Fixed spot price estimate (historical data unavailable)"
		} else {
			estimationMethod = "Historical spot prices from same period last year"
		}
	} else {
		estimatedSpotPrices, err = EstimateSpotPricesForPeriod(startDate, endDate, priceArea)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate spot prices: %v", err)
		}
		estimationMethod = "Fixed spot price estimate"
	}

	// Calculate statistics
	totalEstimated := 0.0
	for _, hourly := range estimatedConsumption {
		totalEstimated += hourly.Consumption
	}

	avgHourlyConsumption := totalEstimated / float64(len(estimatedConsumption))

	var totalSpotPrice float64
	for _, price := range estimatedSpotPrices {
		totalSpotPrice += energinet.ConvertToKWh(price.SpotPriceDKK)
	}
	avgSpotPrice := totalSpotPrice / float64(len(estimatedSpotPrices))

	return &AcontoEstimation{
		EstimatedConsumption: estimatedConsumption,
		EstimatedSpotPrices:  estimatedSpotPrices,
		EstimationMethod:     estimationMethod,
		TotalEstimatedkWh:    totalEstimated,
		AvgHourlyConsumption: avgHourlyConsumption,
		AvgSpotPrice:         avgSpotPrice,
		HoursInPeriod:        len(estimatedConsumption),
	}, nil
}

// CreateHybridEstimation laver en hybrid beregning med estimeret forbrug + blandet spotpriser
func CreateHybridEstimation(
	estimatedAnnualVolume int,
	period Period,
	frequency BillingFrequency,
	refreshToken string,
	meterPointId string,
	priceArea string,
	fixedSpotPrice float64, // Fast pris for estimerede timer (DKK/kWh)
) (*HybridEstimation, error) {

	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	now := time.Now().In(copenhagen)

	// Definer split-tidspunktet for spotpriser (nu minus 2 dage, rounded til nærmeste time)
	spotPriceSplitDateTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, copenhagen).AddDate(0, 0, -2)

	// STEP 1: Generer ALT forbrug baseret på annual consumption (ikke faktisk forbrug)
	// Dette sikrer at aconto altid baserer sig på estimatet
	allEstimatedConsumption, err := EstimateConsumptionForPeriod(estimatedAnnualVolume, period.Start, period.End, frequency)
	if err != nil {
		return nil, fmt.Errorf("kunne ikke estimere forbrug for periode: %v", err)
	}

	// STEP 2: Hent faktiske spotpriser fra periodens start til nu minus 2 dage
	var actualSpotPrices []energinet.SpotPriceRecord
	var estimatedSpotPrices []energinet.SpotPriceRecord

	if spotPriceSplitDateTime.After(period.Start) {
		// Hent faktiske spotpriser for den del hvor de er tilgængelige
		actualSpotPrices, err = FetchSpotPricesForPeriod(period.Start, spotPriceSplitDateTime, priceArea)
		if err != nil {
			return nil, fmt.Errorf("kunne ikke hente faktiske spotpriser: %v", err)
		}

		// Generer faste spotpriser for resten af perioden
		estimatedSpotPrices = generateFixedSpotPricesForPeriod(
			spotPriceSplitDateTime,
			period.End,
			priceArea,
			fixedSpotPrice,
		)
	} else {
		// Hele perioden er i fremtiden - brug kun faste priser
		estimatedSpotPrices = generateFixedSpotPricesForPeriod(
			period.Start,
			period.End,
			priceArea,
			fixedSpotPrice,
		)
	}

	// STEP 3: Kombiner spotpriser
	combinedSpotPrices := append(actualSpotPrices, estimatedSpotPrices...)

	// STEP 4: Beregn statistikker
	totalEstimatedkWh := 0.0
	for _, hourly := range allEstimatedConsumption {
		totalEstimatedkWh += hourly.Consumption
	}

	// STEP 5: Del forbrugsdata op i "faktisk" og "estimeret" baseret på tidspunkt (kun til visning)
	// Men begge dele baserer sig på annual consumption estimation
	currentDateTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, copenhagen)

	var pastConsumption []eloverblik.HourlyConsumption
	var futureConsumption []eloverblik.HourlyConsumption
	pastTotalKWh := 0.0
	futureTotalKWh := 0.0

	for _, hourly := range allEstimatedConsumption {
		if hourly.DateTime.Before(currentDateTime) {
			// Mark som "faktisk periode" selvom det er estimeret
			hourlyPast := hourly
			hourlyPast.Quality = "ESTIMATED_PAST"
			pastConsumption = append(pastConsumption, hourlyPast)
			pastTotalKWh += hourly.Consumption
		} else {
			// Mark som "fremtidig periode"
			hourlyFuture := hourly
			hourlyFuture.Quality = "ESTIMATED_FUTURE"
			futureConsumption = append(futureConsumption, hourlyFuture)
			futureTotalKWh += hourly.Consumption
		}
	}

	// STEP 6: Lav estimations-objekt
	estimation := &HybridEstimation{
		ActualConsumption:    pastConsumption,         // "Faktisk periode" men estimeret forbrug
		ActualSpotPrices:     actualSpotPrices,        // Faktiske spotpriser
		ActualTotalKWh:       pastTotalKWh,            // Estimeret forbrug for faktisk periode
		EstimatedConsumption: futureConsumption,       // Fremtidig periode
		EstimatedSpotPrices:  estimatedSpotPrices,     // Faste spotpriser
		EstimatedTotalKWh:    futureTotalKWh,          // Estimeret forbrug for fremtidige timer
		CombinedConsumption:  allEstimatedConsumption, // Alt forbrug (estimeret)
		CombinedSpotPrices:   combinedSpotPrices,      // Faktiske + faste spotpriser
		TotalEstimatedkWh:    totalEstimatedkWh,       // Total estimeret forbrug
		SplitDateTime:        currentDateTime,         // Nuværende tidspunkt
		ActualHours:          len(pastConsumption),    // Timer i "faktisk" periode
		EstimatedHours:       len(futureConsumption),  // Timer i fremtidige periode
		FixedSpotPrice:       fixedSpotPrice,
		EstimationMethod: fmt.Sprintf("Hybrid aconto: %d timer med faktiske spotpriser + %d timer med fast pris (alt forbrug estimeret)",
			len(actualSpotPrices),
			len(estimatedSpotPrices)),
	}

	return estimation, nil
}

// generateEstimatedConsumptionForRemainingPeriod laver jævnt fordelt forbrug for resten af perioden
func generateEstimatedConsumptionForRemainingPeriod(startTime, endTime time.Time, totalKWh float64) ([]eloverblik.HourlyConsumption, error) {
	duration := endTime.Sub(startTime)
	hoursRemaining := int(duration.Hours())

	if hoursRemaining <= 0 {
		return []eloverblik.HourlyConsumption{}, nil
	}

	avgHourlyConsumption := totalKWh / float64(hoursRemaining)

	var estimatedData []eloverblik.HourlyConsumption
	currentTime := startTime

	for currentTime.Before(endTime) {
		estimatedData = append(estimatedData, eloverblik.HourlyConsumption{
			DateTime:    currentTime,
			Consumption: avgHourlyConsumption,
			Quality:     "ESTIMATED_HYBRID",
		})
		currentTime = currentTime.Add(1 * time.Hour)
	}

	return estimatedData, nil
}

// generateFixedSpotPricesForPeriod laver faste spotpriser for den estimerede periode
func generateFixedSpotPricesForPeriod(startTime, endTime time.Time, priceArea string, fixedPrice float64) []energinet.SpotPriceRecord {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")

	// Convert til MWh for SpotPriceRecord struktur
	fixedPriceMWh := fixedPrice * 1000.0

	var spotPrices []energinet.SpotPriceRecord
	currentTime := startTime

	for currentTime.Before(endTime) {
		hourUTC := currentTime.UTC().Format("2006-01-02T15:04:05")
		hourDK := currentTime.In(copenhagen).Format("2006-01-02T15:04:05")

		spotPrices = append(spotPrices, energinet.SpotPriceRecord{
			HourUTC:      hourUTC,
			HourDK:       hourDK,
			PriceArea:    priceArea,
			SpotPriceDKK: fixedPriceMWh,
			SpotPriceEUR: fixedPriceMWh / 7.45,
		})

		currentTime = currentTime.Add(1 * time.Hour)
	}

	return spotPrices
}

// GetUserFixedSpotPrice spørger brugeren om fast spotpris for estimering
func GetUserFixedSpotPrice() float64 {
	utils.PrintInfo("Hybrid beregning kræver en fast spotpris for de resterende timer.")

	options := []string{
		"0.50 DKK/kWh (lav)",
		"0.61 DKK/kWh (medium)",
		"0.75 DKK/kWh (høj)",
		"Indtast selv",
	}

	choice := utils.GetSimpleChoice("Vælg fast spotpris for estimering", options)

	switch choice {
	case 0:
		return 0.50
	case 1:
		return 0.61
	case 2:
		return 0.75
	default:
		// Indtast selv - du kan implementere dette senere
		utils.PrintInfo("Bruger standard: 0.61 DKK/kWh")
		return 0.75
	}
}

// DisplayAcontoEstimationSummary shows a summary of the aconto estimation
// This function's only purpose is printing, so it's allowed to use utils.Print*
func DisplayAcontoEstimationSummary(estimation *AcontoEstimation, period Period) {
	utils.PrintInfo("=== ACONTO ESTIMATION SUMMARY ===")
	utils.PrintInfo(fmt.Sprintf("Period: %s", period.Label))
	utils.PrintInfo(fmt.Sprintf("Estimation method: %s", estimation.EstimationMethod))
	utils.PrintInfo(fmt.Sprintf("Total estimated consumption: %.2f kWh", estimation.TotalEstimatedkWh))
	utils.PrintInfo(fmt.Sprintf("Average hourly consumption: %.4f kWh", estimation.AvgHourlyConsumption))
	utils.PrintInfo(fmt.Sprintf("Hours in period: %d", estimation.HoursInPeriod))
	utils.PrintInfo(fmt.Sprintf("Average estimated spot price: %.3f DKK/kWh", estimation.AvgSpotPrice))
	utils.PrintWarning("Note: This is an estimate using industry-standard monthly/quarterly division")
	fmt.Println()
}

// DisplayHybridEstimationSummary viser sammendrag af hybrid beregning
func DisplayHybridEstimationSummary(estimation *HybridEstimation, period Period) {
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")

	// Beregn spotpris split-punkt (2 dage før nuværende tidspunkt)
	spotPriceSplitDateTime := estimation.SplitDateTime.AddDate(0, 0, -2)

	utils.PrintInfo("=== HYBRID ACONTO BEREGNING SAMMENDRAG ===")
	utils.PrintInfo(fmt.Sprintf("Periode: %s", period.Label))
	utils.PrintInfo(fmt.Sprintf("Nuværende tidspunkt: %s", estimation.SplitDateTime.In(copenhagen).Format("2006-01-02 15:04")))
	utils.PrintInfo(fmt.Sprintf("Spotpris split-punkt: %s", spotPriceSplitDateTime.In(copenhagen).Format("2006-01-02 15:04")))
	fmt.Println()

	utils.PrintInfo("FORBRUGTE TIMER (estimeret forbrug + faktiske spotpriser):")
	utils.PrintInfo(fmt.Sprintf("Antal timer: %d", estimation.ActualHours))
	utils.PrintInfo(fmt.Sprintf("Estimeret forbrug: %.2f kWh", estimation.ActualTotalKWh))
	utils.PrintInfo(fmt.Sprintf("Faktiske spotpriser: %d timer", len(estimation.ActualSpotPrices)))
	if len(estimation.ActualSpotPrices) > 0 {
		actualAvgSpot := calculateAverageSpotPrice(estimation.ActualSpotPrices)
		utils.PrintInfo(fmt.Sprintf("Gennemsnitlig faktisk spotpris: %.3f DKK/kWh", actualAvgSpot))
	}
	fmt.Println()

	utils.PrintInfo("FREMTIDIGE TIMER (estimeret forbrug + fast spotpris):")
	utils.PrintInfo(fmt.Sprintf("Antal timer: %d", estimation.EstimatedHours))
	utils.PrintInfo(fmt.Sprintf("Estimeret forbrug: %.2f kWh", estimation.EstimatedTotalKWh))
	utils.PrintInfo(fmt.Sprintf("Faste spotpriser: %d timer", len(estimation.EstimatedSpotPrices)))
	utils.PrintInfo(fmt.Sprintf("Fast spotpris: %.3f DKK/kWh", estimation.FixedSpotPrice))
	fmt.Println()

	utils.PrintInfo("TOTALT:")
	utils.PrintInfo(fmt.Sprintf("Samlet estimeret forbrug: %.2f kWh", estimation.TotalEstimatedkWh))
	utils.PrintInfo(fmt.Sprintf("Total timer: %d", len(estimation.CombinedConsumption)))
	utils.PrintInfo(fmt.Sprintf("Metode: %s", estimation.EstimationMethod))
	utils.PrintWarning("Note: Alt forbrug er estimeret baseret på årligt forbrug (aconto princip)")
	fmt.Println()
}

// calculateAverageSpotPrice beregner gennemsnitlig spotpris fra en liste
func calculateAverageSpotPrice(spotPrices []energinet.SpotPriceRecord) float64 {
	if len(spotPrices) == 0 {
		return 0.0
	}

	total := 0.0
	for _, price := range spotPrices {
		total += energinet.ConvertToKWh(price.SpotPriceDKK)
	}

	return total / float64(len(spotPrices))
}

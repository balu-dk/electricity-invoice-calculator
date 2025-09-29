package main

import (
	"electricity-invoice-calculator/lib/billing"
	"electricity-invoice-calculator/lib/eloverblik"
	"electricity-invoice-calculator/lib/energinet"
	"electricity-invoice-calculator/lib/utils"
	"fmt"
	"log"
	"time"
)

// displayMeterPoint formats meter point info for user display
func displayMeterPoint(mp eloverblik.MeterPoint, index int) string {
	address := fmt.Sprintf("%s %s", mp.StreetName, mp.BuildingNumber)
	if mp.FloorId != "" {
		address += fmt.Sprintf(", %s", mp.FloorId)
	}
	if mp.RoomId != "" {
		address += fmt.Sprintf(" %s", mp.RoomId)
	}

	// Use console.go color formatting
	return utils.FormatMeterPoint(
		mp.ID,
		mp.BalanceSupplier,
		mp.Consumer,
		address,
		mp.PostCode,
		mp.City,
		index,
	)
}

// authenticateUser handles the complete authentication flow
func authenticateUser() string {
	utils.ClearConsole()

	utils.PrintAction("Getting authentication token...")
	jwtToken, err := eloverblik.LoadAuthToken("auth.json")
	if err != nil {
		log.Fatal("Failed to load JWT token: ", err)
	}

	utils.PrintAction("Getting refresh token...")
	refreshToken, err := eloverblik.GetRefreshToken(jwtToken)
	if err != nil {
		log.Fatal("Failed to get refresh token: ", err)
	}

	return refreshToken
}

// selectMeterPoint gets and displays meter points for user selection
func selectMeterPoint(refreshToken string) eloverblik.MeterPoint {
	utils.PrintAction("Getting meter points...")
	meterPoints, err := eloverblik.GetMeterPoints(refreshToken)
	if err != nil {
		log.Fatal("Failed to get meter points: ", err)
	}

	if len(meterPoints) == 0 {
		log.Fatal("No meter points found.")
	}

	utils.ClearConsole()

	// Format options for display
	title := fmt.Sprintf("Found %d meter point(s):", len(meterPoints))
	formattedOptions := make([]string, len(meterPoints))
	for i, mp := range meterPoints {
		formattedOptions[i] = displayMeterPoint(mp, i)
	}

	selectedIndex := utils.GetUserChoice(title, formattedOptions)
	selectedMeterPoint := meterPoints[selectedIndex]

	utils.PrintSuccess(fmt.Sprintf("✓ Selected meter point: %s", selectedMeterPoint.ID))
	return selectedMeterPoint
}

// getGridOperatorInfo fetches grid operator details for the selected meter point
func getGridOperatorInfo(refreshToken string, meterPoint eloverblik.MeterPoint) eloverblik.MeterPointDetails {
	utils.PrintAction("Getting detailed information...")
	gridOperator, err := eloverblik.GetMeterPointDetails(refreshToken, meterPoint.ID)
	if err != nil {
		log.Fatal("Failed to get grid operator details:", err)
	}

	return gridOperator
}

// displayMeterPointDetails shows the selected meter point and grid operator details
func displayMeterPointDetails(meterPoint eloverblik.MeterPoint, gridOperator eloverblik.MeterPointDetails) {
	utils.ClearConsole()

	utils.PrintInfo("Meter Point Details:")
	fmt.Printf("ID: %s\n", meterPoint.ID)
	fmt.Printf("Address: %s %s, %s %s\n",
		meterPoint.StreetName,
		meterPoint.BuildingNumber,
		meterPoint.PostCode,
		meterPoint.City)
	fmt.Printf("Grid Operator: %s\n", gridOperator.Name)
	fmt.Printf("Estimated Annual Volume: %d kWh\n", gridOperator.EstimatedAnnualVolume)
}

func main() {
	// Authentication
	refreshToken := authenticateUser()

	// Meter point selection
	selectedMeterPoint := selectMeterPoint(refreshToken)

	// Get grid operator info
	gridOperator := getGridOperatorInfo(refreshToken, selectedMeterPoint)

	// Display details
	displayMeterPointDetails(selectedMeterPoint, gridOperator)

	utils.ClearConsole()

	// Get calculation type
	calculationType := billing.GetCalculationType()
	utils.PrintSuccess(fmt.Sprintf("Selected calculation type: %s", calculationType))

	// Get billing frequency
	frequency := billing.GetBillingFrequency()
	utils.PrintSuccess(fmt.Sprintf("Selected billing frequency: %s", frequency))

	// Period selection with calculation type
	utils.PrintAction("Generating available periods...")

	// Parse consumer start date
	consumerStartDate, err := time.Parse("2006-01-02T15:04:05.000Z", selectedMeterPoint.ConsumerStartDate)
	if err != nil {
		log.Fatal("Failed to parse consumer start date:", err)
	}

	// Generate available periods
	periods, err := billing.GenerateAvailablePeriods(consumerStartDate, frequency, calculationType)
	if err != nil {
		log.Fatal("Failed to generate periods:", err)
	}

	if len(periods) == 0 {
		if calculationType == billing.Historical {
			utils.PrintWarning("No complete historical periods available for calculation yet.")
		} else {
			utils.PrintWarning("No aconto periods available.")
		}
		return
	}

	// Let user select period
	selectedPeriod := billing.SelectPeriod(periods)

	utils.ClearConsole()

	// Display selected period
	billing.DisplaySelectedPeriod(selectedPeriod)

	// NEW: Determine actual period type based on dates
	periodType := billing.DeterminePeriodType(selectedPeriod, calculationType)
	utils.PrintInfo(fmt.Sprintf("Detected period type: %s", periodType))

	// Load grid companies mapping
	gridMapping, err := billing.LoadGridCompaniesMapping("lib/billing/grid_companies.json")
	if err != nil {
		log.Fatal("Failed to load grid companies mapping:", err)
	}

	// Find price area for grid operator
	priceArea, err := billing.FindPriceArea(gridOperator.Name, gridMapping)
	if err != nil {
		log.Fatal("Failed to find price area:", err)
	}

	utils.PrintInfo(fmt.Sprintf("Grid operator: %s, Price area: %s", gridOperator.Name, priceArea))

	// NEW: Branch based on detected period type
	var consumptionData []eloverblik.HourlyConsumption
	var totalConsumption float64
	var spotPrices []energinet.SpotPriceRecord

	switch periodType {
	case billing.PeriodHistorical:
		// Historical calculation
		utils.PrintAction("Fetching actual consumption data...")
		consumptionData, err = eloverblik.GetConsumptionForPeriod(
			refreshToken,
			selectedMeterPoint.ID,
			selectedPeriod.Start,
			selectedPeriod.End,
		)
		if err != nil {
			log.Fatal("Failed to get consumption data:", err)
		}

		totalConsumption = eloverblik.GetTotalConsumption(consumptionData)
		summary := eloverblik.FormatConsumptionSummary(consumptionData)
		utils.PrintInfo(summary)

		// Fetch real spot prices
		utils.PrintAction("Fetching spot prices...")
		spotPrices, err = billing.FetchSpotPricesForPeriod(selectedPeriod.Start, selectedPeriod.End, priceArea)
		if err != nil {
			log.Fatal("Failed to fetch spot prices:", err)
		}
		utils.PrintInfo(fmt.Sprintf("Fetched %d spot price records", len(spotPrices)))

	case billing.PeriodAconto:
		// Pure aconto calculation
		utils.PrintAction("Estimating consumption for aconto calculation...")

		// Ask about spot price method
		spotPriceOptions := []string{
			"Use fixed estimate (614.029 DKK/MWh)",
			"Use historical prices from same period last year",
		}
		spotPriceChoice := utils.GetSimpleChoice("How should we estimate spot prices?", spotPriceOptions)
		useHistoricalPrices := (spotPriceChoice == 1)

		// Create aconto estimation
		acontoEstimation, err := billing.CreateAcontoEstimation(
			gridOperator.EstimatedAnnualVolume,
			selectedPeriod.Start,
			selectedPeriod.End,
			priceArea,
			frequency,
			useHistoricalPrices,
		)
		if err != nil {
			log.Fatal("Failed to create aconto estimation:", err)
		}

		// Display estimation summary
		billing.DisplayAcontoEstimationSummary(acontoEstimation, selectedPeriod)

		// Use estimated data
		consumptionData = acontoEstimation.EstimatedConsumption
		totalConsumption = acontoEstimation.TotalEstimatedkWh
		spotPrices = acontoEstimation.EstimatedSpotPrices

		utils.PrintInfo(fmt.Sprintf("Generated %d hours of estimated consumption data", len(consumptionData)))

	case billing.PeriodHybrid:
		// NEW: Hybrid calculation
		utils.PrintAction("Performing hybrid calculation (actual + estimated data)...")

		// Get fixed spot price for estimated portion
		fixedSpotPrice := billing.GetUserFixedSpotPrice()

		// Create hybrid estimation
		hybridEstimation, err := billing.CreateHybridEstimation(
			gridOperator.EstimatedAnnualVolume,
			selectedPeriod,
			frequency,
			refreshToken,
			selectedMeterPoint.ID,
			priceArea,
			fixedSpotPrice,
		)
		if err != nil {
			log.Fatal("Failed to create hybrid estimation:", err)
		}

		// Display hybrid summary
		billing.DisplayHybridEstimationSummary(hybridEstimation, selectedPeriod)

		// Use combined data
		consumptionData = hybridEstimation.CombinedConsumption
		totalConsumption = hybridEstimation.TotalEstimatedkWh
		spotPrices = hybridEstimation.CombinedSpotPrices

		utils.PrintInfo(fmt.Sprintf("Using %d hours of combined data (%d actual + %d estimated)",
			len(consumptionData), hybridEstimation.ActualHours, hybridEstimation.EstimatedHours))
	}

	// Continue with shared calculation logic
	hourlyBreakdown := eloverblik.GetConsumptionByHour(consumptionData)
	utils.PrintInfo(fmt.Sprintf("Data spread across %d different hours of day", len(hourlyBreakdown)))

	utils.PrintAction("Fetching charges (tariffs and subscriptions)...")
	chargesData, err := eloverblik.GetCharges(refreshToken, selectedMeterPoint.ID)
	if err != nil {
		log.Fatal("Failed to get charges data:", err)
	}

	utils.PrintSuccess("Successfully retrieved charges data")

	utils.PrintAction("Calculating complete electricity bill with spot prices...")

	// Set supplier price based on calculation type
	supplierPrice := 0.0 // No supplier cost for aconto/hybrid calculations
	if periodType == billing.PeriodHistorical {
		supplierPrice = 0.02 // 2 øre per kWh for historical calculations only
	}

	hourlyTariffCosts := billing.CalculateAllHourlyTariffs(consumptionData, chargesData, supplierPrice, spotPrices)

	// Get summaries
	tariffSummary := billing.SummarizeTariffCosts(hourlyTariffCosts)
	totalTariffCosts := billing.GetTotalTariffCosts(hourlyTariffCosts)
	totalSupplierCosts := billing.GetTotalSupplierCosts(hourlyTariffCosts)
	totalSpotCosts := billing.GetTotalSpotCosts(hourlyTariffCosts)

	// Calculate subscription costs
	var totalSubscriptionCost float64
	subscriptionBreakdown := make(map[string]float64)

	var monthsInPeriod float64
	if frequency == billing.Monthly {
		monthsInPeriod = 1.0
	} else { // Quarterly
		monthsInPeriod = 3.0
	}

	for _, subscription := range chargesData.Subscriptions {
		monthlyCost := subscription.Price * float64(subscription.Quantity)
		periodCost := monthlyCost * monthsInPeriod
		subscriptionBreakdown[subscription.Name] = periodCost
		totalSubscriptionCost += periodCost
	}

	// Calculate total bill
	totalBill := totalTariffCosts + totalSubscriptionCost

	// Calculate VAT (25% in Denmark)
	const VAT_RATE = 0.25
	totalVAT := totalBill * VAT_RATE
	totalBillWithVAT := totalBill + totalVAT

	// Display results with appropriate title based on period type
	utils.ClearConsole()
	switch periodType {
	case billing.PeriodHistorical:
		utils.PrintSuccess(fmt.Sprintf("=== HISTORICAL ELECTRICITY BILL FOR %s ===", selectedPeriod.Label))
	case billing.PeriodAconto:
		utils.PrintSuccess(fmt.Sprintf("=== ACONTO ELECTRICITY BILL ESTIMATE FOR %s ===", selectedPeriod.Label))
	case billing.PeriodHybrid:
		utils.PrintSuccess(fmt.Sprintf("=== HYBRID ELECTRICITY BILL FOR %s ===", selectedPeriod.Label))
	}
	fmt.Println()

	// Consumption summary
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	utils.PrintInfo("CONSUMPTION SUMMARY:")
	utils.PrintInfo(fmt.Sprintf("Period: %s to %s",
		selectedPeriod.Start.In(copenhagen).Format("2006-01-02"),
		selectedPeriod.End.AddDate(0, 0, -1).In(copenhagen).Format("2006-01-02")))

	switch periodType {
	case billing.PeriodHistorical:
		utils.PrintInfo(fmt.Sprintf("Total consumption: %.2f kWh (actual)", totalConsumption))
	case billing.PeriodAconto:
		utils.PrintInfo(fmt.Sprintf("Total consumption: %.2f kWh (estimated)", totalConsumption))
		utils.PrintInfo(fmt.Sprintf("Based on estimated annual volume: %d kWh", gridOperator.EstimatedAnnualVolume))
	case billing.PeriodHybrid:
		utils.PrintInfo(fmt.Sprintf("Total consumption: %.2f kWh (actual + estimated)", totalConsumption))
		utils.PrintInfo(fmt.Sprintf("Based on estimated annual volume: %d kWh", gridOperator.EstimatedAnnualVolume))
	}
	fmt.Println()

	// Usage-based charges
	utils.PrintInfo("USAGE-BASED CHARGES:")
	for name, cost := range tariffSummary {
		utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", name, cost))
	}
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Elleverandør", totalSupplierCosts))
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Spotpris", totalSpotCosts))
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Total usage charges", totalTariffCosts))
	fmt.Println()

	// Fixed charges
	utils.PrintInfo("FIXED MONTHLY CHARGES:")
	for name, cost := range subscriptionBreakdown {
		utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", name, cost))
	}
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Total subscriptions", totalSubscriptionCost))
	fmt.Println()

	// Total bill
	utils.PrintInfo("BILL SUMMARY:")
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Subtotal (excluding VAT)", totalBill))
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "VAT (25%)", totalVAT))
	utils.PrintSuccess(fmt.Sprintf("%-30s: %8.2f DKK", "TOTAL INCLUDING VAT", totalBillWithVAT))
	fmt.Println()

	utils.PrintInfo(fmt.Sprintf("Average cost per kWh (incl. VAT): %.3f DKK", totalBillWithVAT/totalConsumption))

	// Add appropriate disclaimers
	if periodType == billing.PeriodAconto || periodType == billing.PeriodHybrid {
		fmt.Println()
		utils.PrintWarning("ESTIMATION DISCLAIMER:")
		utils.PrintWarning("This includes estimates based on industry-standard monthly/quarterly division.")
		utils.PrintWarning("Actual consumption patterns and spot prices may vary significantly.")
		utils.PrintWarning("Use this estimate for budgeting purposes only.")
	}
}

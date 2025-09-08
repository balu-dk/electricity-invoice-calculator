package main

import (
	"electricity-invoice-calculator/lib/billing"
	"electricity-invoice-calculator/lib/eloverblik"
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
func getGridOperatorInfo(refreshToken string, meterPoint eloverblik.MeterPoint) eloverblik.MeterPointGridOperator {
	utils.PrintAction("Getting detailed information...")
	gridOperator, err := eloverblik.GetMeterPointGridOperator(refreshToken, meterPoint.ID)
	if err != nil {
		log.Fatal("Failed to get grid operator details:", err)
	}

	return gridOperator
}

// displayMeterPointDetails shows the selected meter point and grid operator details
func displayMeterPointDetails(meterPoint eloverblik.MeterPoint, gridOperator eloverblik.MeterPointGridOperator) {
	utils.ClearConsole()

	utils.PrintInfo("Meter Point Details:")
	fmt.Printf("ID: %s\n", meterPoint.ID)
	fmt.Printf("Address: %s %s, %s %s\n",
		meterPoint.StreetName,
		meterPoint.BuildingNumber,
		meterPoint.PostCode,
		meterPoint.City)
	fmt.Printf("Grid Operator: %s\n", gridOperator.Name)
}

// getBillingFrequency asks user for their billing frequency
func getBillingFrequency() billing.BillingFrequency {
	billingOptions := []string{"Monthly", "Quarterly"}
	freqChoice := utils.GetSimpleChoice("How are you billed?", billingOptions)

	if freqChoice == 0 {
		return billing.Monthly
	}
	return billing.Quarterly
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

	// Get billing frequency
	frequency := getBillingFrequency()

	utils.ClearConsole()

	utils.PrintSuccess(fmt.Sprintf("Selected billing frequency: %s", frequency))

	// Period selection
	utils.PrintAction("Generating available periods...")

	// Parse consumer start date (you'll need to add ConsumerStartDate to your MeterPoint struct)
	consumerStartDate, err := time.Parse("2006-01-02T15:04:05.000Z", selectedMeterPoint.ConsumerStartDate)
	if err != nil {
		log.Fatal("Failed to parse consumer start date:", err)
	}

	// Generate available periods
	periods, err := billing.GenerateAvailablePeriods(consumerStartDate, frequency)
	if err != nil {
		log.Fatal("Failed to generate periods:", err)
	}

	if len(periods) == 0 {
		utils.PrintWarning("No complete periods available for calculation yet.")
		return
	}

	// Let user select period
	selectedPeriod := billing.SelectPeriod(periods)

	utils.ClearConsole()

	// Display selected period
	billing.DisplaySelectedPeriod(selectedPeriod)

	utils.PrintAction("Fetching consumption data...")
	consumptionData, err := eloverblik.GetConsumptionForPeriod(
		refreshToken,
		selectedMeterPoint.ID,
		selectedPeriod.Start,
		selectedPeriod.End,
	)
	if err != nil {
		log.Fatal("Failed to get consumption data:", err)
	}

	utils.PrintSuccess(fmt.Sprintf("Successfully processed %d hours of consumption data", len(consumptionData)))

	summary := eloverblik.FormatConsumptionSummary(consumptionData)
	utils.PrintInfo(summary)

	totalConsumption := eloverblik.GetTotalConsumption(consumptionData)
	utils.PrintInfo(fmt.Sprintf("Total consumption for period: %.2f kWh", totalConsumption))

	hourlyBreakdown := eloverblik.GetConsumptionByHour(consumptionData)
	utils.PrintInfo(fmt.Sprintf("Data spread across %d different hours of day", len(hourlyBreakdown)))

	utils.PrintAction("Fetching charges (tariffs and subscriptions)...")
	chargesData, err := eloverblik.GetCharges(refreshToken, selectedMeterPoint.ID)
	if err != nil {
		log.Fatal("Failed to get charges data:", err)
	}

	utils.PrintSuccess("Successfully retrieved charges data")

	utils.PrintAction("Calculating complete electricity bill with spot prices...")

	// Load grid companies mapping
	gridMapping, err := billing.LoadGridCompaniesMapping("lib/billing/grid_companies.json")
	if err != nil {
		log.Fatal("Failed to load grid companies mapping:", err)
	}

	// Find price area for your grid operator
	priceArea, err := billing.FindPriceArea(gridOperator.Name, gridMapping)
	if err != nil {
		log.Fatal("Failed to find price area:", err)
	}

	utils.PrintInfo(fmt.Sprintf("Grid operator: %s, Price area: %s", gridOperator.Name, priceArea))

	// Fetch spot prices for the period
	spotPrices, err := billing.FetchSpotPricesForPeriod(selectedPeriod.Start, selectedPeriod.End, priceArea)
	if err != nil {
		log.Fatal("Failed to fetch spot prices:", err)
	}

	utils.PrintInfo(fmt.Sprintf("Fetched %d spot price records", len(spotPrices)))

	// Calculate all hourly costs with spot prices
	supplierPrice := 0.02 // 2 øre per kWh - you can make this interactive later
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

	// Display results
	utils.ClearConsole()
	utils.PrintSuccess(fmt.Sprintf("=== COMPLETE ELECTRICITY BILL FOR %s ===", selectedPeriod.Label))
	fmt.Println()

	// Consumption summary
	copenhagen, _ := time.LoadLocation("Europe/Copenhagen")
	utils.PrintInfo("CONSUMPTION SUMMARY:")
	utils.PrintInfo(fmt.Sprintf("Period: %s to %s",
		selectedPeriod.Start.In(copenhagen).Format("2006-01-02"),
		selectedPeriod.End.AddDate(0, 0, -1).In(copenhagen).Format("2006-01-02")))
	utils.PrintInfo(fmt.Sprintf("Total consumption: %.2f kWh", totalConsumption))
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
	utils.PrintSuccess(fmt.Sprintf("TOTAL ELECTRICITY BILL: %.2f DKK", totalBill))
	utils.PrintInfo(fmt.Sprintf("Average cost per kWh: %.3f DKK", totalBill/totalConsumption))

	// Calculate VAT (25% in Denmark)
	const VAT_RATE = 0.25
	totalVAT := totalBill * VAT_RATE
	totalBillWithVAT := totalBill + totalVAT

	// Display results
	utils.ClearConsole()
	utils.PrintSuccess(fmt.Sprintf("=== COMPLETE ELECTRICITY BILL FOR %s ===", selectedPeriod.Label))
	fmt.Println()

	// Total bill
	utils.PrintInfo("BILL SUMMARY:")
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "Subtotal (excluding VAT)", totalBill))
	utils.PrintInfo(fmt.Sprintf("%-30s: %8.2f DKK", "VAT (25%)", totalVAT))
	utils.PrintSuccess(fmt.Sprintf("%-30s: %8.2f DKK", "TOTAL INCLUDING VAT", totalBillWithVAT))
	fmt.Println()

	utils.PrintInfo(fmt.Sprintf("Average cost per kWh (incl. VAT): %.3f DKK", totalBillWithVAT/totalConsumption))
}

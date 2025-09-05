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

	// Calcu
}

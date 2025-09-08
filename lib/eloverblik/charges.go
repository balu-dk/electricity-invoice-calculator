package eloverblik

import (
	"electricity-invoice-calculator/lib/utils"
	"encoding/json"
	"fmt"
	"time"
)

// Price represents a single price entry with position and price
type Price struct {
	Position string  `json:"position"`
	Price    float64 `json:"price"`
}

// Subscription represents a monthly subscription fee
type Subscription struct {
	Price         float64 `json:"price"`
	Quantity      int     `json:"quantity"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Owner         string  `json:"owner"`
	ValidFromDate string  `json:"validFromDate"`
	ValidToDate   *string `json:"validToDate"`
	PeriodType    string  `json:"periodType"`
}

// Tariff represents a usage-based tariff
type Tariff struct {
	Prices        []Price `json:"prices"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Owner         string  `json:"owner"`
	ValidFromDate string  `json:"validFromDate"`
	ValidToDate   *string `json:"validToDate"`
	PeriodType    string  `json:"periodType"`
}

// ChargesResult contains the charges information for a meter point
type ChargesResult struct {
	Fees            []interface{}  `json:"fees"`
	MeteringPointId string         `json:"meteringPointId"`
	Subscriptions   []Subscription `json:"subscriptions"`
	Tariffs         []Tariff       `json:"tariffs"`
}

// ChargesAPIResponse represents the full API response
type ChargesAPIResponse struct {
	Result []struct {
		Result     ChargesResult `json:"result"`
		Success    bool          `json:"success"`
		ErrorCode  int           `json:"errorCode"`
		ErrorText  string        `json:"errorText"`
		ID         string        `json:"id"`
		StackTrace interface{}   `json:"stackTrace"`
	} `json:"result"`
}

// HourlyCharge represents the calculated charge for a specific hour
type HourlyCharge struct {
	DateTime      time.Time
	Consumption   float64
	SpotPrice     float64            // DKK per kWh
	TariffCharges map[string]float64 // tariff name -> charge amount
	TotalCharge   float64
}

// BillingCalculation contains the complete billing calculation for a period
type BillingCalculation struct {
	Period                string
	TotalConsumption      float64
	TotalSpotCost         float64
	TotalTariffCosts      map[string]float64
	MonthlySubscriptions  map[string]float64
	TotalSubscriptionCost float64
	TotalCost             float64
	HourlyCharges         []HourlyCharge
}

// GetCharges fetches tariff and subscription information for a meter point
func GetCharges(refreshToken, meterPointId string) (*ChargesResult, error) {
	url := APIEndpoint + "meteringpoints/meteringpoint/getcharges"

	body := []byte(fmt.Sprintf(`{
		"meteringPoints": {
			"meteringPoint": ["%s"]
		}
	}`, meterPointId))

	response, err := utils.MakeRequestWithToken("POST", url, refreshToken, body)
	if err != nil {
		return nil, fmt.Errorf("failed to get charges: %v", err)
	}

	if err = utils.ValidateStatusOK(response); err != nil {
		return nil, err
	}

	var apiResponse ChargesAPIResponse
	err = json.Unmarshal(response.Body, &apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse charges JSON: %v", err)
	}

	// Process the response and return the charges directly
	if len(apiResponse.Result) == 0 {
		return nil, fmt.Errorf("no charges data found in response")
	}

	resultItem := apiResponse.Result[0]

	if !resultItem.Success {
		return nil, fmt.Errorf("API call failed: %s (error code: %d)", resultItem.ErrorText, resultItem.ErrorCode)
	}

	return &resultItem.Result, nil
}

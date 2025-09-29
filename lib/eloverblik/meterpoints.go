package eloverblik

import (
	"electricity-invoice-calculator/lib/utils"
	"encoding/json"
	"fmt"
)

var APIEndpoint string = "https://api.eloverblik.dk/customerapi/api/"

type MeterPoint struct {
	ID                string `json:"meteringPointId"`
	Consumer          string `json:"firstConsumerPartyName"`
	BalanceSupplier   string `json:"balanceSupplierName"`
	PostCode          string `json:"postcode"`
	City              string `json:"cityName"`
	StreetName        string `json:"streetName"`
	BuildingNumber    string `json:"buildingNumber"`
	FloorId           string `json:"floorId"`
	RoomId            string `json:"roomId"`
	ConsumerStartDate string `json:"consumerStartDate"`
}

type MeterPointDetails struct {
	Name                   string `json:"gridOperatorName"`
	ID                     string `json:"gridOperatorID"`
	EstimatedAnnualVolume  int    `json:"estimatedAnnualVolume,string"`
	GridAreaIdentification string `json:"meteringGridAreaIdentification"`
}

type APIResponse struct {
	Result []MeterPoint `json:"result"`
}

type DetailedAPIResponse struct {
	Result []struct {
		Result MeterPointDetails `json:"result"`
	} `json:"result"`
}

// Requests meter points and returns respnse as list of MeterPoint
func GetMeterPoints(refreshToken string) ([]MeterPoint, error) {
	url := APIEndpoint + "meteringpoints/meteringpoints"

	response, err := utils.MakeRequestWithToken("GET", url, refreshToken, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get meter points: %v", err)
	}

	if err = utils.ValidateStatusOK(response); err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	err = json.Unmarshal(response.Body, &apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse meter points JSON: %v", err)
	}

	return apiResponse.Result, nil
}

// Requests for Meter Point (extra) details
// and returns Grid Operator details as MeterPointDetails
func GetMeterPointDetails(refreshToken, meterPointId string) (MeterPointDetails, error) {
	url := APIEndpoint + "meteringpoints/meteringpoint/getdetails"

	body := []byte(fmt.Sprintf(`{
		"meteringPoints": {
			"meteringPoint": ["%s"]
		}
	}`, meterPointId))
	response, err := utils.MakeRequestWithToken("POST", url, refreshToken, body)
	if err != nil {
		return MeterPointDetails{}, fmt.Errorf("failed to get meter point grid operator: %v", err)
	}

	if err = utils.ValidateStatusOK(response); err != nil {
		return MeterPointDetails{}, err
	}

	var apiResponse DetailedAPIResponse
	err = json.Unmarshal(response.Body, &apiResponse)
	if err != nil {
		return MeterPointDetails{}, fmt.Errorf("failed to parse meter point grid operator JSON: %v", err)
	}

	// Check if we have any results
	if len(apiResponse.Result) == 0 {
		return MeterPointDetails{}, fmt.Errorf("no meter point details found")
	}

	return apiResponse.Result[0].Result, nil
}

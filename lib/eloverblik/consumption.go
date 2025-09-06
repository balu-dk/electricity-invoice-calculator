package eloverblik

import (
	"electricity-invoice-calculator/lib/utils"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Point struct {
	Position string `json:"position"`
	Quantity string `json:"out_Quantity.quantity"`
	Quality  string `json:"out_Quantity.quality"`
}

type Period struct {
	Resolution   string `json:"resolution"`
	TimeInterval struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"timeInterval"`
	Point []Point `json:"Point"`
}

type MarketEvaluationPoint struct {
	MRID struct {
		CodingScheme string `json:"codingScheme"`
		Name         string `json:"name"`
	} `json:"mRID"`
}

type TimeSeries struct {
	MRID                  string                `json:"mRID"`
	BusinessType          string                `json:"businessType"`
	CurveType             string                `json:"curveType"`
	MeasurementUnitName   string                `json:"measurement_Unit.name"`
	MarketEvaluationPoint MarketEvaluationPoint `json:"MarketEvaluationPoint"`
	Period                []Period              `json:"Period"`
}

type SenderMarketParticipant struct {
	Name string `json:"sender_MarketParticipant.name"`
	MRID struct {
		CodingScheme interface{} `json:"codingScheme"`
		Name         interface{} `json:"name"`
	} `json:"sender_MarketParticipant.mRID"`
}

type EnergyData struct {
	MRID                    string `json:"mRID"`
	CreatedDateTime         string `json:"createdDateTime"`
	SenderMarketParticipant SenderMarketParticipant
	PeriodTimeInterval      struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period.timeInterval"`
	TimeSeries []TimeSeries `json:"TimeSeries"`
}

type ResultItem struct {
	EnergyData EnergyData  `json:"MyEnergyData_MarketDocument"`
	Success    bool        `json:"success"`
	ErrorCode  int         `json:"errorCode"`
	ErrorText  string      `json:"errorText"`
	ID         string      `json:"id"`
	StackTrace interface{} `json:"stackTrace"`
}

type ConsumptionAPIResponse struct {
	Result []ResultItem `json:"result"`
}

type HourlyConsumption struct {
	DateTime    time.Time
	Consumption float64
	Quality     string
}

func GetConsumptionData(refreshToken, meterPointId string, startDate, endDate time.Time) (*ConsumptionAPIResponse, error) {
	url := APIEndpoint + "meterdata/gettimeseries/" + startDate.Format("2006-01-02") + "/" + endDate.Format("2006-01-02") + "/Hour"

	body := []byte(fmt.Sprintf(`{
		"meteringPoints": {
			"meteringPoint": ["%s"]
		}
	}`, meterPointId))

	response, err := utils.MakeRequestWithToken("POST", url, refreshToken, body)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumption data: %v", err)
	}

	if err = utils.ValidateStatusOK(response); err != nil {
		return nil, err
	}

	var apiResponse ConsumptionAPIResponse
	err = json.Unmarshal(response.Body, &apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse consumption data JSON: %v", err)
	}

	return &apiResponse, nil
}

// ProcessConsumptionData converts raw API response to structured hourly consumption data
func ProcessConsumptionData(response *ConsumptionAPIResponse) ([]HourlyConsumption, error) {
	if len(response.Result) == 0 {
		return nil, fmt.Errorf("no consumption data found in response")
	}

	resultItem := response.Result[0]

	// Check if the API call was successful
	if !resultItem.Success {
		return nil, fmt.Errorf("API call failed: %s (error code: %d)", resultItem.ErrorText, resultItem.ErrorCode)
	}

	energyData := resultItem.EnergyData
	if len(energyData.TimeSeries) == 0 {
		return nil, fmt.Errorf("no time series data found")
	}

	timeSeries := energyData.TimeSeries[0]
	utils.PrintInfo(fmt.Sprintf("Processing TimeSeries with %d periods", len(timeSeries.Period)))

	var hourlyConsumptions []HourlyConsumption

	for _, period := range timeSeries.Period {
		// Parse the start date of the period
		startTime, err := time.Parse("2006-01-02T15:04:05Z", period.TimeInterval.Start)
		if err != nil {
			return nil, fmt.Errorf("failed to parse period start time: %v", err)
		}

		for _, point := range period.Point {
			// Parse position as integer
			position, err := strconv.Atoi(point.Position)
			if err != nil {
				return nil, fmt.Errorf("failed to parse position: %v", err)
			}

			// Parse consumption quantity
			quantity, err := strconv.ParseFloat(point.Quantity, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse quantity: %v", err)
			}

			// Calculate the actual datetime for this hour
			// Position 1 = startTime, Position 2 = startTime + 1 hour, etc.
			hourDateTime := startTime.Add(time.Duration(position-1) * time.Hour)

			hourlyConsumption := HourlyConsumption{
				DateTime:    hourDateTime,
				Consumption: quantity,
				Quality:     point.Quality,
			}

			hourlyConsumptions = append(hourlyConsumptions, hourlyConsumption)
		}
	}

	return hourlyConsumptions, nil
}

// GetConsumptionForPeriod is a convenience function that fetches and processes consumption data
func GetConsumptionForPeriod(refreshToken, meterPointId string, startDate, endDate time.Time) ([]HourlyConsumption, error) {
	response, err := GetConsumptionData(refreshToken, meterPointId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	hourlyConsumptions, err := ProcessConsumptionData(response)
	if err != nil {
		return nil, err
	}

	return hourlyConsumptions, nil
}

// GetTotalConsumption calculates total consumption for a period
func GetTotalConsumption(hourlyConsumptions []HourlyConsumption) float64 {
	var total float64
	for _, hourly := range hourlyConsumptions {
		total += hourly.Consumption
	}
	return total
}

// GetConsumptionByHour returns consumption data grouped by hour of day (0-23)
func GetConsumptionByHour(hourlyConsumptions []HourlyConsumption) map[int][]float64 {
	hourlyMap := make(map[int][]float64)

	for _, hourly := range hourlyConsumptions {
		hour := hourly.DateTime.Hour()
		hourlyMap[hour] = append(hourlyMap[hour], hourly.Consumption)
	}

	return hourlyMap
}

// FormatConsumptionSummary creates a formatted summary of consumption data
func FormatConsumptionSummary(hourlyConsumptions []HourlyConsumption) string {
	if len(hourlyConsumptions) == 0 {
		return "No consumption data available"
	}

	totalConsumption := GetTotalConsumption(hourlyConsumptions)
	totalHours := len(hourlyConsumptions)
	avgHourly := totalConsumption / float64(totalHours)

	startDate := hourlyConsumptions[0].DateTime.Format("2006-01-02")
	endDate := hourlyConsumptions[len(hourlyConsumptions)-1].DateTime.Format("2006-01-02")

	return fmt.Sprintf(
		"Consumption Summary:\n"+
			"Period: %s to %s\n"+
			"Total hours: %d\n"+
			"Total consumption: %.2f kWh\n"+
			"Average hourly consumption: %.3f kWh",
		startDate,
		endDate,
		totalHours,
		totalConsumption,
		avgHourly,
	)
}

package energinet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type SpotPriceRecord struct {
	HourUTC      string  `json:"HourUTC"`
	HourDK       string  `json:"HourDK"`
	PriceArea    string  `json:"PriceArea"`
	SpotPriceDKK float64 `json:"SpotPriceDKK"`
	SpotPriceEUR float64 `json:"SpotPriceEUR"`
}

type APIResponse struct {
	Total   int               `json:"total"`
	Limit   int               `json:"limit"`
	Dataset string            `json:"dataset"`
	Records []SpotPriceRecord `json:"records"`
}

// Builds URL with correct parameters for API convention
func buildURL(startDate, endDate string, priceAreas []string) string {
	baseURL := "https://api.energidataservice.dk/dataset/Elspotprices"

	params := url.Values{}

	params.Add("offset", "0")
	params.Add("start", startDate)
	params.Add("end", endDate)
	params.Add("sort", "HourUTC DESC")

	if len(priceAreas) > 0 {
		areas := strings.Join(priceAreas, `","`)
		filter := fmt.Sprintf(`{"PriceArea":["%s"]}`, areas)
		params.Add("filter", filter)
	}

	return baseURL + "?" + params.Encode()
}

// Gets spot prices from public Energinet API
func GetSpotPrices(startDate, endDate string, priceAreas []string) ([]SpotPriceRecord, error) {
	apiURL := buildURL(startDate, endDate, priceAreas)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %v", err)
	}

	var apiResp APIResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %v", err)
	}

	return apiResp.Records, nil
}

// Converts from MWh to kWh
func ConvertToKWh(pricePerMWh float64) float64 {
	return pricePerMWh / 1000
}

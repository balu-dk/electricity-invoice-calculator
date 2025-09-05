package eloverblik

import (
	"electricity-invoice-calculator/lib/utils"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type AuthConfig struct {
	JWTToken string `json:"jwtToken"`
}

type TokenResponse struct {
	Token string `json:"result"`
}

// Reads static generated Eloverblik JWT token from file
func LoadAuthToken(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("could not open %s: %v", filename, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %v", filename, err)
	}

	var auth AuthConfig
	err = json.Unmarshal(data, &auth)
	if err != nil {
		return "", fmt.Errorf("could not parse JSON in %s: %v", filename, err)
	}

	if auth.JWTToken == "" {
		return "", fmt.Errorf("jwtToken is empty in %s", filename)
	}

	return auth.JWTToken, nil
}

// Returns refresh token from JWT token
func GetRefreshToken(jwtToken string) (string, error) {
	url := "https://api.eloverblik.dk/customerapi/api/token"

	response, err := utils.MakeRequestWithToken("GET", url, jwtToken, nil)
	if err != nil {
		return "", fmt.Errorf("token request failed: %v", err)
	}

	err = utils.ValidateStatusOK(response)
	if err != nil {
		return "", err
	}

	var tokenResp TokenResponse
	err = json.Unmarshal(response.Body, &tokenResp)
	if err != nil {
		return "", fmt.Errorf("could not parse JSON: %v", err)
	}

	return tokenResp.Token, nil
}

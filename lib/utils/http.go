package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type HTTPResponse struct {
	Body       []byte
	StatusCode int
}

func MakeRequestWithToken(method, url, token string, body []byte) (*HTTPResponse, error) {
	headers := make(map[string]string)
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	headers["Content-Type"] = "application/json"

	return MakeRequest(method, url, headers, body)
}

func MakeRequest(method, url string, headers map[string]string, body []byte) (*HTTPResponse, error) {
	var bodyReader io.Reader
	// Reads and inteperets body and gets it
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	// Creates request
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %v", err)
	}

	// Add auth headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %v", err)
	}

	return &HTTPResponse{
		Body:       responseBody,
		StatusCode: resp.StatusCode,
	}, nil
}

func ValidateStatusOK(response *HTTPResponse) error {
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", response.StatusCode)
	}
	return nil
}

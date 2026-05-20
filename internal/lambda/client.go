package lambda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrTimeout = errors.New("request timed out")

type RequestBody struct {
	Duration       string `json:"duration"`
	MessageID      string `json:"messageId"`
	Reason         string `json:"reason"`
	Requester      string `json:"requester"`
	RequesterEmail string `json:"requesterEmail"`
	Scope          string `json:"scope"`
}

type PostResult struct {
	AlreadyMember bool
	Message       string
}

type StatusResponse struct {
	User   string   `json:"user"`
	Groups []string `json:"groups"`
}

func Post(lambdaURL, bearerToken string, body RequestBody) (*PostResult, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, lambdaURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Timeout() {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
		return &PostResult{AlreadyMember: true, Message: result.Message}, nil
	case http.StatusAccepted, http.StatusServiceUnavailable:
		return &PostResult{AlreadyMember: false}, nil
	default:
		return nil, fmt.Errorf("lambda returned HTTP %d", resp.StatusCode)
	}
}

func GetStatus(statusURL, bearerToken string) (*StatusResponse, error) {
	req, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request: %w", err)
	}
	defer resp.Body.Close()

	var sr StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}
	return &sr, nil
}

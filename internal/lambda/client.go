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

func Post(lambdaURL, bearerToken string, body RequestBody) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, lambdaURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Timeout() {
			return ErrTimeout
		}
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		return fmt.Errorf("lambda returned HTTP %d", resp.StatusCode)
	}
	return nil
}

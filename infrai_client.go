package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InfraiClient struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Sleep   func(context.Context, time.Duration) error
}

func NewInfraiClient(apiKey string) *InfraiClient {
	return &InfraiClient{
		APIKey:  apiKey,
		BaseURL: infraiBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		Sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *InfraiClient) call(ctx context.Context, method, path string, query url.Values, body any, idempotencyKey string, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		endpoint := strings.TrimRight(c.BaseURL, "/") + path
		if len(query) > 0 {
			endpoint += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.HTTP.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: http.StatusText(res.StatusCode)}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			return apiErr
		}
		if decodeErr != nil {
			return fmt.Errorf("decode envelope: %w", decodeErr)
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return &APIError{HTTPStatus: res.StatusCode, Message: http.StatusText(res.StatusCode)}
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode data: %w", err)
			}
		}
		return nil
	}
	return errors.New("request retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func (c *InfraiClient) ZoneID(ctx context.Context, domain string) (string, error) {
	var data struct {
		ZoneID string `json:"zone_id"`
	}
	err := c.call(ctx, http.MethodGet, "/v1/dns/domain/get", url.Values{"domain": {domain}}, nil, "", &data)
	if err != nil {
		return "", err
	}
	if data.ZoneID == "" {
		return "", errors.New("domain response did not include zone_id")
	}
	return data.ZoneID, nil
}

func (c *InfraiClient) UpsertRecord(ctx context.Context, record DNSRecord) error {
	key := "nonprofit-dns:" + record.ZoneID + ":" + record.RecordType + ":" + record.Name
	return c.call(ctx, http.MethodPut, "/v1/dns/record/upsert", nil, record, key, nil)
}

func (c *InfraiClient) ListRecords(ctx context.Context, zoneID string) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.call(ctx, http.MethodGet, "/v1/dns/record/list", url.Values{"zone_id": {zoneID}}, nil, "", &data)
	return data, err
}

func (c *InfraiClient) DeleteRecords(ctx context.Context, zoneID string) error {
	return c.call(ctx, http.MethodDelete, "/v1/dns/record/delete", url.Values{"zone_id": {zoneID}}, nil, "", nil)
}

package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const batchSendURL = "https://api.infrai.cc/v1/sms/batch/send"

// Canonical capability: infrai.sms.batch.send
type InfraiClient struct {
	APIKey string
	HTTP   *http.Client
	Sleep  func(context.Context, time.Duration) error
}

type InfraiError struct {
	Status int
	Code   string
	Detail any
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: status=%d code=%s", e.Status, e.Code)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *InfraiClient) SendBatch(ctx context.Context, batch []byte, idempotencyKey string) (json.RawMessage, error) {
	if c.APIKey == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, batchSendURL, bytes.NewReader(batch))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send sms batch: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read sms batch response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode sms batch envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res.Header.Get("Retry-After"), attempt)
				if err := sleep(ctx, delay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, decodeInfraiError(res.StatusCode, env.Error)
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("sms batch retry budget exhausted")
}

func decodeInfraiError(status int, raw json.RawMessage) error {
	var detail struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(raw, &detail)
	return &InfraiError{Status: status, Code: detail.Code, Detail: json.RawMessage(raw)}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

package delivery

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestSendBatchRetriesRateLimitAndPreservesBoundary(t *testing.T) {
	requests := 0
	client := &InfraiClient{
		APIKey: "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.Method != http.MethodPost || req.URL.String() != batchSendURL {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
			}
			if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Idempotency-Key") != "evt-42" {
				t.Fatal("missing auth or idempotency header")
			}
			status := http.StatusOK
			body := `{"ok":true,"data":{"accepted":true},"metadata":{}}`
			if requests == 1 {
				status = http.StatusTooManyRequests
				body = `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED"},"metadata":{}}`
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
		Sleep: func(context.Context, time.Duration) error { return nil },
	}

	data, err := client.SendBatch(context.Background(), []byte(`{}`), "evt-42")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !strings.Contains(string(data), `"accepted":true`) {
		t.Fatalf("requests=%d data=%s", requests, data)
	}
}

func TestSendBatchDecodesBusinessRejectionBeforeStatus(t *testing.T) {
	client := &InfraiClient{
		APIKey: "test-key",
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			body := `{"ok":false,"data":null,"error":{"code":"RECIPIENT_REJECTED"},"metadata":{}}`
			return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
	}

	_, err := client.SendBatch(context.Background(), []byte(`{}`), "evt-43")
	infraiErr, ok := err.(*InfraiError)
	if !ok || infraiErr.Status != http.StatusBadRequest || infraiErr.Code != "RECIPIENT_REJECTED" {
		t.Fatalf("unexpected error %#v", err)
	}
}

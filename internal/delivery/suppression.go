package delivery

import (
	"fmt"
	"strings"
	"sync"
)

type EventKind string

const (
	SubscriberUpdated EventKind = "subscriber_updated"
	AssetReady        EventKind = "asset_ready"
	ContentProcessed  EventKind = "content_processed"
)

type Event struct {
	ID          string
	Kind        EventKind
	Phone       string
	OptedOut    bool
	InfraiBatch []byte
}

type Decision struct {
	Send   bool   `json:"send"`
	Reason string `json:"reason"`
}

type Suppressions struct {
	mu     sync.RWMutex
	phones map[string]struct{}
}

func NewSuppressions() *Suppressions {
	return &Suppressions{phones: make(map[string]struct{})}
}

func (s *Suppressions) Apply(event Event) (Decision, error) {
	phone, err := normalizePhone(event.Phone)
	if err != nil {
		return Decision{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if event.Kind == SubscriberUpdated {
		if event.OptedOut {
			s.phones[phone] = struct{}{}
			return Decision{Reason: "subscriber_opted_out"}, nil
		}
		delete(s.phones, phone)
		return Decision{Reason: "subscriber_active"}, nil
	}

	if event.Kind != AssetReady && event.Kind != ContentProcessed {
		return Decision{}, fmt.Errorf("unsupported event kind %q", event.Kind)
	}
	if _, suppressed := s.phones[phone]; suppressed {
		return Decision{Reason: "suppressed"}, nil
	}
	if len(event.InfraiBatch) == 0 {
		return Decision{}, fmt.Errorf("infrai batch document is required")
	}
	return Decision{Send: true, Reason: "approved"}, nil
}

func normalizePhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "+") || len(value) < 8 {
		return "", fmt.Errorf("phone must use E.164 format")
	}
	for _, r := range value[1:] {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("phone must use E.164 format")
		}
	}
	return value, nil
}

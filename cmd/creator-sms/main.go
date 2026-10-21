package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/creator-sms-suppression/internal/delivery"
)

type eventRequest struct {
	ID          string             `json:"id"`
	Kind        delivery.EventKind `json:"kind"`
	Phone       string             `json:"phone"`
	OptedOut    bool               `json:"opted_out"`
	InfraiBatch json.RawMessage    `json:"infrai_batch"`
}

type server struct {
	suppressions *delivery.Suppressions
	infrai       *delivery.InfraiClient
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	s := &server{
		suppressions: delivery.NewSuppressions(),
		infrai:       &delivery.InfraiClient{APIKey: apiKey},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.handleEvent)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("creator SMS gate listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var input eventRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event"})
		return
	}

	decision, err := s.suppressions.Apply(delivery.Event{
		ID: input.ID, Kind: input.Kind, Phone: input.Phone,
		OptedOut: input.OptedOut, InfraiBatch: input.InfraiBatch,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !decision.Send {
		writeJSON(w, http.StatusOK, decision)
		return
	}

	data, err := s.infrai.SendBatch(r.Context(), input.InfraiBatch, input.ID)
	if err != nil {
		var apiErr *delivery.InfraiError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
			writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Detail})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "sms delivery transport error"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"decision": decision, "delivery": data})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

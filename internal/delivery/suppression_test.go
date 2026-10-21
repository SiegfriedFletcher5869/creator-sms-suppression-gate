package delivery

import "testing"

func TestSuppressionDecision(t *testing.T) {
	tests := []struct {
		name       string
		setup      []Event
		event      Event
		wantSend   bool
		wantReason string
	}{
		{
			name:       "active subscriber receives asset delivery",
			event:      Event{ID: "evt-1", Kind: AssetReady, Phone: "+14155550100", InfraiBatch: []byte(`{}`)},
			wantSend:   true,
			wantReason: "approved",
		},
		{
			name:       "opted out subscriber suppresses processed content update",
			setup:      []Event{{ID: "evt-2", Kind: SubscriberUpdated, Phone: "+14155550101", OptedOut: true}},
			event:      Event{ID: "evt-3", Kind: ContentProcessed, Phone: "+14155550101"},
			wantReason: "suppressed",
		},
		{
			name:       "resubscribe permits later delivery",
			setup:      []Event{{Kind: SubscriberUpdated, Phone: "+14155550102", OptedOut: true}, {Kind: SubscriberUpdated, Phone: "+14155550102"}},
			event:      Event{ID: "evt-4", Kind: AssetReady, Phone: "+14155550102", InfraiBatch: []byte(`{}`)},
			wantSend:   true,
			wantReason: "approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSuppressions()
			for _, event := range tt.setup {
				if _, err := store.Apply(event); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			got, err := store.Apply(tt.event)
			if err != nil {
				t.Fatal(err)
			}
			if got.Send != tt.wantSend || got.Reason != tt.wantReason {
				t.Fatalf("got %+v, want send=%v reason=%q", got, tt.wantSend, tt.wantReason)
			}
		})
	}
}

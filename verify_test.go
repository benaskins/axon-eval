package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Verify(t *testing.T) {
	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/runs/run-123/summary" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(VerifyResult{
				RunID:                "run-123",
				Messages:             6,
				ToolInvocations:      2,
				Conversations:        3,
				Memories:             1,
				RelationshipSnapshots: 1,
				Consolidations:       0,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer analyticsServer.Close()

	client := &Client{
		config:     Config{AnalyticsURL: analyticsServer.URL},
		httpClient: http.DefaultClient,
	}

	result, err := client.Verify("run-123")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if result.RunID != "run-123" {
		t.Errorf("expected run_id 'run-123', got %q", result.RunID)
	}
	if result.Messages != 6 {
		t.Errorf("expected 6 messages, got %d", result.Messages)
	}
	if result.ToolInvocations != 2 {
		t.Errorf("expected 2 tool invocations, got %d", result.ToolInvocations)
	}
	if result.Conversations != 3 {
		t.Errorf("expected 3 conversations, got %d", result.Conversations)
	}
}

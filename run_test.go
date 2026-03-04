package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestClient_Run(t *testing.T) {
	var mu sync.Mutex
	var analyticsEvents []map[string]interface{}
	var chatRequests []map[string]interface{}

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/user-created":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/chat/sync" && r.Method == http.MethodPost:
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			chatRequests = append(chatRequests, req)
			mu.Unlock()

			// Return a sync response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response":    "Hello! I'm doing well.",
				"duration_ms": 100,
				"tools_used":  []string{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer chatServer.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"user_id":       "test-user-id",
			"session_token": "test-token",
		})
	}))
	defer authServer.Close()

	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/events" && r.Method == http.MethodPost {
			var events []map[string]interface{}
			json.NewDecoder(r.Body).Decode(&events)
			mu.Lock()
			analyticsEvents = append(analyticsEvents, events...)
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			return
		}
		http.NotFound(w, r)
	}))
	defer analyticsServer.Close()

	client, err := NewClient(Config{
		AuthURL:      authServer.URL,
		ChatURL:      chatServer.URL,
		AnalyticsURL: analyticsServer.URL,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	run, err := client.Run("smoke test", []Scenario{
		Conversation("greeting", []Message{
			{Role: "user", Content: "Hello, how are you?"},
		}),
		Conversation("multi-turn", []Message{
			{Role: "user", Content: "What is Go?"},
			{Role: "user", Content: "Tell me more."},
		}),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if run.ID == "" {
		t.Error("expected run ID")
	}

	// Should have sent 3 chat requests (1 for greeting, 2 for multi-turn)
	mu.Lock()
	defer mu.Unlock()

	if len(chatRequests) != 3 {
		t.Errorf("expected 3 chat requests, got %d", len(chatRequests))
	}

	// Should have run_started and run_completed events
	if len(analyticsEvents) < 2 {
		t.Fatalf("expected at least 2 analytics events, got %d", len(analyticsEvents))
	}

	firstEvent := analyticsEvents[0]
	if firstEvent["type"] != "run_started" {
		t.Errorf("expected run_started, got %v", firstEvent["type"])
	}
	if firstEvent["run_id"] != run.ID {
		t.Errorf("expected run_id %q, got %v", run.ID, firstEvent["run_id"])
	}

	lastEvent := analyticsEvents[len(analyticsEvents)-1]
	if lastEvent["type"] != "run_completed" {
		t.Errorf("expected run_completed, got %v", lastEvent["type"])
	}

	// Check responses were collected
	if len(run.Responses) != 2 {
		t.Errorf("expected 2 scenario responses, got %d", len(run.Responses))
	}
}

func TestClient_Run_SetsRunIDHeader(t *testing.T) {
	var mu sync.Mutex
	var runIDHeaders []string

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/user-created":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/chat/sync" && r.Method == http.MethodPost:
			mu.Lock()
			runIDHeaders = append(runIDHeaders, r.Header.Get("X-Axon-Run-Id"))
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response":    "ok",
				"duration_ms": 50,
				"tools_used":  []string{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer chatServer.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"user_id":       "test-user",
			"session_token": "test-token",
		})
	}))
	defer authServer.Close()

	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer analyticsServer.Close()

	client, err := NewClient(Config{
		AuthURL:      authServer.URL,
		ChatURL:      chatServer.URL,
		AnalyticsURL: analyticsServer.URL,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	run, err := client.Run("test", []Scenario{
		Conversation("single", []Message{{Role: "user", Content: "Hi"}}),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(runIDHeaders) != 1 {
		t.Fatalf("expected 1 chat request, got %d", len(runIDHeaders))
	}
	if runIDHeaders[0] != run.ID {
		t.Errorf("expected X-Axon-Run-Id %q, got %q", run.ID, runIDHeaders[0])
	}
}

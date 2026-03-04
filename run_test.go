package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestClient_Run(t *testing.T) {
	var mu sync.Mutex
	var analyticsEvents []map[string]interface{}
	var chatRequests []map[string]interface{}
	convCounter := 0

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/user-created":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent/conversations" && r.Method == http.MethodPost:
			mu.Lock()
			convCounter++
			id := fmt.Sprintf("conv-%d", convCounter)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":         id,
				"agent_slug": "xagent",
			})
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

	// Each chat request should have a real conversation_id from the server
	for i, req := range chatRequests {
		cid, _ := req["conversation_id"].(string)
		if cid == "" {
			t.Errorf("chat request %d missing conversation_id", i)
		}
	}

	// Greeting scenario (1 message) should use conv-1
	if cid := chatRequests[0]["conversation_id"]; cid != "conv-1" {
		t.Errorf("greeting should use conv-1, got %v", cid)
	}

	// Multi-turn scenario (2 messages) should both use conv-2
	if cid := chatRequests[1]["conversation_id"]; cid != "conv-2" {
		t.Errorf("multi-turn msg 1 should use conv-2, got %v", cid)
	}
	if cid := chatRequests[2]["conversation_id"]; cid != "conv-2" {
		t.Errorf("multi-turn msg 2 should use conv-2, got %v", cid)
	}
}

func TestClient_EmitEvalResult(t *testing.T) {
	var mu sync.Mutex
	var analyticsEvents []map[string]interface{}

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/user-created":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
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
		if r.URL.Path == "/api/events" && r.Method == http.MethodPost {
			var events []map[string]interface{}
			json.NewDecoder(r.Body).Decode(&events)
			mu.Lock()
			analyticsEvents = append(analyticsEvents, events...)
			mu.Unlock()
		}
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

	grade := &ScenarioGrade{
		Scenario: "greeting",
		Results: []CriterionResult{
			{Criterion: "min_length", Pass: true, Score: 1, Reason: "ok"},
			{Criterion: "llm_judge", Pass: false, Score: 0, Reason: "skipped"},
		},
		Passed: 1,
		Failed: 1,
		Total:  2,
	}

	chatResult := ChatResult{
		Response:   "Hello there!",
		DurationMs: 2847,
		ToolsUsed:  []string{"check_weather"},
	}

	err = client.EmitEvalResult("run-test-123", grade, chatResult)
	if err != nil {
		t.Fatalf("EmitEvalResult failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Find the eval_result event
	var found bool
	for _, ev := range analyticsEvents {
		if ev["type"] == "eval_result" {
			found = true
			if ev["run_id"] != "run-test-123" {
				t.Errorf("expected run_id 'run-test-123', got %v", ev["run_id"])
			}
			if ev["scenario"] != "greeting" {
				t.Errorf("expected scenario 'greeting', got %v", ev["scenario"])
			}
			if ev["response"] != "Hello there!" {
				t.Errorf("expected response, got %v", ev["response"])
			}
			if int(ev["passed"].(float64)) != 1 {
				t.Errorf("expected passed=1, got %v", ev["passed"])
			}
		}
	}
	if !found {
		t.Error("expected eval_result event, none found")
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
		case r.URL.Path == "/api/agents/xagent/conversations" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "conv-1"})
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

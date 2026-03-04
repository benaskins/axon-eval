package eval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Evaluate(t *testing.T) {
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/user-created":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/agents/xagent/conversations" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "conv-test"})
		case r.URL.Path == "/api/chat/sync" && r.Method == http.MethodPost:
			var req map[string]interface{}
			json.NewDecoder(r.Body).Decode(&req)
			messages := req["messages"].([]interface{})
			lastMsg := messages[len(messages)-1].(map[string]interface{})
			msg := lastMsg["content"].(string)

			response := "I don't know."
			if msg == "What's 2+2?" {
				response = "The answer is 4."
			} else if msg == "Tell me about Melbourne" {
				response = "Melbourne is the capital of Victoria, Australia. It is known for its vibrant arts scene, diverse culture, excellent coffee, and unpredictable weather. The city hosts major sporting events including the Australian Open and the Melbourne Cup."
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response":    response,
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

	eval, err := client.Evaluate("response quality", []EvalScenario{
		{
			Messages: []Message{{Role: "user", Content: "What's 2+2?"}},
			Check:    ResponseContains("4"),
		},
		{
			Messages: []Message{{Role: "user", Content: "Tell me about Melbourne"}},
			Check:    ResponseMinLength(100),
		},
		{
			Messages: []Message{{Role: "user", Content: "Unknown query"}},
			Check:    ResponseContains("specific answer"),
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if eval.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", eval.Passed)
	}
	if eval.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", eval.Failed)
	}
	if len(eval.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(eval.Results))
	}

	if !eval.Results[0].Pass {
		t.Error("expected first scenario to pass")
	}
	if !eval.Results[1].Pass {
		t.Error("expected second scenario to pass")
	}
	if eval.Results[2].Pass {
		t.Error("expected third scenario to fail")
	}
}

func TestResponseContains(t *testing.T) {
	check := ResponseContains("hello")
	pass, reason := check("hello world")
	if !pass {
		t.Errorf("expected pass, got: %s", reason)
	}

	pass, reason = check("goodbye")
	if pass {
		t.Error("expected fail")
	}
	if reason == "" {
		t.Error("expected reason on failure")
	}
}

func TestResponseMinLength(t *testing.T) {
	check := ResponseMinLength(10)
	pass, _ := check("short")
	if pass {
		t.Error("expected fail for short response")
	}

	pass, _ = check("this is a longer response")
	if !pass {
		t.Error("expected pass for long response")
	}
}

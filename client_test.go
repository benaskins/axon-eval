package eval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewClient_Setup(t *testing.T) {
	var serviceUserCalls atomic.Int32
	var userCreatedCalls atomic.Int32
	var agentPutCalls atomic.Int32

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/service-user" && r.Method == http.MethodPost {
			serviceUserCalls.Add(1)
			json.NewEncoder(w).Encode(map[string]string{
				"user_id":       "test-user-id",
				"session_token": "test-token-abc",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer authServer.Close()

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/user-created" && r.Method == http.MethodPost {
			userCreatedCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/agents/xagent" && r.Method == http.MethodPut {
			agentPutCalls.Add(1)

			// Verify auth header
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "test-token-abc" {
				t.Errorf("expected session cookie 'test-token-abc', got err=%v", err)
			}

			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer chatServer.Close()

	analyticsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if client.userID != "test-user-id" {
		t.Errorf("expected user_id 'test-user-id', got %q", client.userID)
	}
	if client.sessionToken != "test-token-abc" {
		t.Errorf("expected session token 'test-token-abc', got %q", client.sessionToken)
	}

	if serviceUserCalls.Load() != 1 {
		t.Errorf("expected 1 service-user call, got %d", serviceUserCalls.Load())
	}
	if userCreatedCalls.Load() != 1 {
		t.Errorf("expected 1 user-created call, got %d", userCreatedCalls.Load())
	}
	if agentPutCalls.Load() != 1 {
		t.Errorf("expected 1 agent PUT call, got %d", agentPutCalls.Load())
	}
}

func TestNewClient_InvalidAuthURL(t *testing.T) {
	_, err := NewClient(Config{
		AuthURL:      "http://localhost:1", // nothing listening
		ChatURL:      "http://localhost:2",
		AnalyticsURL: "http://localhost:3",
	})
	if err == nil {
		t.Error("expected error for invalid auth URL")
	}
}

package explorer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	loop "github.com/benaskins/axon-loop"
)

// mockLLM implements loop.LLMClient for testing.
type mockLLM struct {
	response string
}

func (m *mockLLM) Chat(_ context.Context, req *loop.Request, fn func(loop.Response) error) error {
	return fn(loop.Response{Content: m.response, Done: true})
}

func TestChat_ReturnsSSE(t *testing.T) {
	s := NewServer(Config{})
	s.SetLLM(&mockLLM{response: "Hello from explorer"})
	handler := s.Handler()

	body := `{"message": "hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %q", contentType)
	}

	if !strings.Contains(w.Body.String(), "Hello from explorer") {
		t.Errorf("expected response content in SSE stream, got:\n%s", w.Body.String())
	}
}

func TestChat_EmptyMessage(t *testing.T) {
	s := NewServer(Config{})
	s.SetLLM(&mockLLM{response: "ok"})
	handler := s.Handler()

	body := `{"message": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty message, got %d", w.Code)
	}
}

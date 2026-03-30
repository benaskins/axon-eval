package explorer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_Health(t *testing.T) {
	s := NewServer(Config{})
	handler := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

package explorer

import (
	"encoding/json"
	"net/http"
)

// Config holds explorer service configuration.
type Config struct {
	Port          string `env:"PORT" envDefault:"8094"`
	ClickHouseURL string `env:"CLICKHOUSE_URL" envDefault:"http://localhost:8123"`
	ModelURL      string `env:"MODEL_URL" envDefault:"http://localhost:8091"`
	Model         string `env:"MODEL" envDefault:"qwen3.5-122b"`
}

// Server is the explorer HTTP server.
type Server struct {
	cfg Config
}

// NewServer creates an explorer server.
func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Handler returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return mux
}

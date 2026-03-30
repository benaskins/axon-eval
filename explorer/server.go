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

const defaultSystemPrompt = `You are an eval data explorer. You help users understand their evaluation results through visualisation and analysis.

You have access to ClickHouse tables containing BFCL benchmark results and autotune experiment data. When a user asks a question:

1. Query the relevant data using your tools
2. Visualise the results, prefer charts over text for comparisons
3. Explain what you see, be curious, notice patterns
4. Point out missing data that would be useful to capture

Always visualise when comparing. Use tables for small datasets (<10 rows), charts for everything else. Pick the chart type that best shows the relationship the user is asking about.`

// Server is the explorer HTTP server.
type Server struct {
	cfg Config
	llm LLMClient
}

// NewServer creates an explorer server.
func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// SetLLM configures the LLM client. Used for testing.
func (s *Server) SetLLM(llm LLMClient) {
	s.llm = llm
}

// Handler returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.Handle("POST /api/chat", &chatHandler{
		llm:          s.llm,
		model:        s.cfg.Model,
		systemPrompt: defaultSystemPrompt,
	})

	return mux
}

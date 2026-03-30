package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/benaskins/axon-eval/explorer"
	"github.com/benaskins/axon-talk/openai"
)

func main() {
	cfg := explorer.Config{
		Port:          envOr("PORT", "8094"),
		ClickHouseURL: envOr("CLICKHOUSE_URL", "http://localhost:8123"),
		ModelURL:      envOr("MODEL_URL", "http://localhost:8091"),
		Model:         envOr("MODEL", "qwen3.5-122b"),
	}

	srv := explorer.NewServer(cfg)
	srv.SetLLM(openai.NewClient(cfg.ModelURL, ""))

	slog.Info("explorer starting", "port", cfg.Port, "model_url", cfg.ModelURL, "model", cfg.Model)
	if err := http.ListenAndServe(":"+cfg.Port, srv.Handler()); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	loop "github.com/benaskins/axon-loop"
)

// LLMClient is the interface for sending chat requests to an LLM.
type LLMClient interface {
	Chat(ctx context.Context, req *loop.Request, fn func(loop.Response) error) error
}

// chatRequest is the JSON body for POST /api/chat.
type chatRequest struct {
	Message string `json:"message"`
}

// chatHandler handles POST /api/chat with SSE streaming.
type chatHandler struct {
	llm         LLMClient
	model       string
	systemPrompt string
}

func (h *chatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	messages := []loop.Message{
		{Role: "system", Content: h.systemPrompt},
		{Role: "user", Content: req.Message},
	}

	llmReq := &loop.Request{
		Model:    h.model,
		Messages: messages,
		Stream:   true,
	}

	err := h.llm.Chat(r.Context(), llmReq, func(resp loop.Response) error {
		if resp.Content != "" {
			data, _ := json.Marshal(map[string]any{
				"type":    "content",
				"content": resp.Content,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		if len(resp.ToolCalls) > 0 {
			for _, tc := range resp.ToolCalls {
				data, _ := json.Marshal(map[string]any{
					"type":      "tool_call",
					"name":      tc.Name,
					"arguments": tc.Arguments,
				})
				fmt.Fprintf(w, "data: %s\n\n", data)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}

		if resp.Done {
			fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		return nil
	})

	if err != nil {
		data, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": err.Error(),
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
}

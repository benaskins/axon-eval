package test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Message is a single message in a conversation scenario.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Scenario is a test scenario to execute during a run.
type Scenario struct {
	Name     string
	Messages []Message
}

// Conversation creates a scenario from a name and messages.
func Conversation(name string, messages []Message) Scenario {
	return Scenario{Name: name, Messages: messages}
}

// Run holds the results of a test run.
type Run struct {
	ID        string
	Responses []ScenarioResult
}

// ChatResult holds the response from a synchronous chat request.
type ChatResult struct {
	Response   string   `json:"response"`
	Thinking   string   `json:"thinking,omitempty"`
	DurationMs int64    `json:"duration_ms"`
	ToolsUsed  []string `json:"tools_used"`
}

// ScenarioResult holds the result of a single scenario.
type ScenarioResult struct {
	Name      string
	Responses []ChatResult
}

func timeNowFormat() string {
	return time.Now().Format("20060102-150405")
}

func randomSuffix() string {
	b := make([]byte, 2)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Run executes a batch of test scenarios, bracketed by run_started/run_completed events.
func (c *Client) Run(description string, scenarios []Scenario) (*Run, error) {
	runID := fmt.Sprintf("run-%s-%s", timeNowFormat(), randomSuffix())

	// Emit run_started
	if err := c.emitRunEvent("run_started", runID, description); err != nil {
		return nil, fmt.Errorf("emit run_started: %w", err)
	}

	run := &Run{ID: runID}

	for _, scenario := range scenarios {
		conversationID, err := c.createConversation()
		if err != nil {
			return nil, fmt.Errorf("scenario %q: create conversation: %w", scenario.Name, err)
		}

		result := ScenarioResult{Name: scenario.Name}
		for _, msg := range scenario.Messages {
			chatResult, err := c.sendChat(msg, runID, conversationID)
			if err != nil {
				return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
			}
			result.Responses = append(result.Responses, *chatResult)
		}
		run.Responses = append(run.Responses, result)
	}

	// Emit run_completed
	if err := c.emitRunEvent("run_completed", runID, description); err != nil {
		return nil, fmt.Errorf("emit run_completed: %w", err)
	}

	return run, nil
}

func (c *Client) emitRunEvent(eventType, runID, description string) error {
	events := []map[string]interface{}{
		{
			"type":        eventType,
			"timestamp":   time.Now().UTC(),
			"run_id":      runID,
			"agent_slug":  c.config.agentSlug(),
			"user_id":     c.userID,
			"description": description,
		},
	}
	body, _ := json.Marshal(events)

	resp, err := c.httpClient.Post(c.config.AnalyticsURL+"/api/events", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, http.StatusAccepted); err != nil {
		return err
	}

	return nil
}

// EmitEvalResult sends an eval_result event to the analytics service.
func (c *Client) EmitEvalResult(runID string, grade *ScenarioGrade, result ChatResult) error {
	criteriaJSON, _ := json.Marshal(grade.Results)
	toolsJSON, _ := json.Marshal(result.ToolsUsed)

	events := []map[string]interface{}{
		{
			"type":        "eval_result",
			"timestamp":   time.Now().UTC(),
			"run_id":      runID,
			"agent_slug":  c.config.agentSlug(),
			"user_id":     c.userID,
			"scenario":    grade.Scenario,
			"response":    result.Response,
			"duration_ms": result.DurationMs,
			"tools_used":  json.RawMessage(toolsJSON),
			"passed":      grade.Passed,
			"failed":      grade.Failed,
			"total":       grade.Total,
			"criteria":    json.RawMessage(criteriaJSON),
		},
	}
	body, _ := json.Marshal(events)

	resp, err := c.httpClient.Post(c.config.AnalyticsURL+"/api/events", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, http.StatusAccepted); err != nil {
		return err
	}

	return nil
}

// createConversation creates a new conversation via the chat service API.
func (c *Client) createConversation() (string, error) {
	req, err := c.authenticatedRequest(http.MethodPost, c.config.ChatURL+"/api/agents/"+c.config.agentSlug()+"/conversations", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, http.StatusCreated); err != nil {
		return "", err
	}

	var conv struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&conv); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return conv.ID, nil
}

func (c *Client) sendChat(msg Message, runID, conversationID string) (*ChatResult, error) {
	chatReq := map[string]interface{}{
		"agent_slug":      c.config.agentSlug(),
		"conversation_id": conversationID,
		"messages": []map[string]string{
			{"role": msg.Role, "content": msg.Content},
		},
	}
	body, _ := json.Marshal(chatReq)

	req, err := c.authenticatedRequest(http.MethodPost, c.config.ChatURL+"/api/chat/sync", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if runID != "" {
		req.Header.Set("X-Axon-Run-Id", runID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}

	var result ChatResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

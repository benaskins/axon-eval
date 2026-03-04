package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// ScenarioResult holds the result of a single scenario.
type ScenarioResult struct {
	Name      string
	Responses []string
}

func timeNowFormat() string {
	return time.Now().Format("20060102-150405")
}

// Run executes a batch of test scenarios, bracketed by run_started/run_completed events.
func (c *Client) Run(description string, scenarios []Scenario) (*Run, error) {
	runID := fmt.Sprintf("run-%s", timeNowFormat())

	// Emit run_started
	if err := c.emitRunEvent("run_started", runID, description); err != nil {
		return nil, fmt.Errorf("emit run_started: %w", err)
	}

	run := &Run{ID: runID}

	for _, scenario := range scenarios {
		result := ScenarioResult{Name: scenario.Name}
		for _, msg := range scenario.Messages {
			response, err := c.sendChat(msg, runID)
			if err != nil {
				return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
			}
			result.Responses = append(result.Responses, response)
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
			"agent_slug":  "xagent",
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

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *Client) sendChat(msg Message, runID string) (string, error) {
	chatReq := map[string]interface{}{
		"agent_slug": "xagent",
		"message":    msg.Content,
		"run_id":     runID,
	}
	body, _ := json.Marshal(chatReq)

	req, err := c.authenticatedRequest(http.MethodPost, c.config.ChatURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.Response, nil
}

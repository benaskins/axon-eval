package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds the URLs for the services that axon-test interacts with.
type Config struct {
	AuthURL      string
	ChatURL      string
	AnalyticsURL string
}

// Client is the main entry point for running test scenarios and evaluations.
// Call NewClient to create one — it handles service user and agent setup.
type Client struct {
	config       Config
	httpClient   *http.Client
	userID       string
	sessionToken string
}

// NewClient creates a test client, setting up the service user and test agent.
// This calls the auth service to create a robot user, notifies the chat service,
// and creates the xagent test agent.
func NewClient(cfg Config) (*Client, error) {
	c := &Client{
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	if err := c.setupServiceUser(); err != nil {
		return nil, fmt.Errorf("setup service user: %w", err)
	}

	if err := c.notifyUserCreated(); err != nil {
		return nil, fmt.Errorf("notify user created: %w", err)
	}

	if err := c.setupTestAgent(); err != nil {
		return nil, fmt.Errorf("setup test agent: %w", err)
	}

	return c, nil
}

func (c *Client) setupServiceUser() error {
	body, _ := json.Marshal(map[string]string{
		"username":     "xagent-runner",
		"display_name": "Test Runner",
	})

	resp, err := c.httpClient.Post(c.config.AuthURL+"/internal/service-user", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		UserID       string `json:"user_id"`
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	c.userID = result.UserID
	c.sessionToken = result.SessionToken
	return nil
}

func (c *Client) notifyUserCreated() error {
	body, _ := json.Marshal(map[string]string{
		"user_id": c.userID,
	})

	resp, err := c.httpClient.Post(c.config.ChatURL+"/internal/user-created", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *Client) setupTestAgent() error {
	agent := map[string]interface{}{
		"name":          "X Agent",
		"system_prompt": "You are a test agent used for infrastructure verification and evaluation.",
		"skills":        []string{"current_time", "web_search", "check_weather", "recall_memory"},
	}
	body, _ := json.Marshal(agent)

	req, err := http.NewRequest(http.MethodPut, c.config.ChatURL+"/api/agents/xagent", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: c.sessionToken})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// authenticatedRequest creates an HTTP request with the session cookie attached.
func (c *Client) authenticatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: c.sessionToken})
	return req, nil
}

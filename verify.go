package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// VerifyResult holds the event counts for a run, queried from the analytics service.
type VerifyResult struct {
	RunID                string `json:"run_id"`
	Messages             int    `json:"messages"`
	ToolInvocations      int    `json:"tool_invocations"`
	Conversations        int    `json:"conversations"`
	Memories             int    `json:"memories"`
	RelationshipSnapshots int   `json:"relationship_snapshots"`
	Consolidations       int    `json:"consolidations"`
}

// Verify queries the analytics service for event counts from a specific run.
func (c *Client) Verify(runID string) (*VerifyResult, error) {
	req, err := c.authenticatedRequest(http.MethodGet, c.config.AnalyticsURL+"/api/runs/"+runID+"/summary", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp, http.StatusOK); err != nil {
		return nil, err
	}

	var result VerifyResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

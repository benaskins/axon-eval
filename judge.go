package test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TextGenerator produces text from a prompt. Matches the axon-memo pattern.
type TextGenerator func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error)

// OllamaJudge implements Judge using an LLM via TextGenerator.
type OllamaJudge struct {
	generate TextGenerator
}

// NewOllamaJudge creates a judge backed by the given text generator.
func NewOllamaJudge(generate TextGenerator) *OllamaJudge {
	return &OllamaJudge{generate: generate}
}

// Grade sends the response, ideal response, and criterion to the LLM judge
// and parses a structured JSON result.
func (j *OllamaJudge) Grade(response, idealResponse, criterion string) (*JudgeResult, error) {
	prompt := buildJudgePrompt(response, idealResponse, criterion)

	text, err := j.generate(context.Background(), prompt, 0.3, 512)
	if err != nil {
		return nil, fmt.Errorf("judge generate: %w", err)
	}

	cleaned := extractJSONFromMarkdown(text)

	var result JudgeResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse judge result: %w (response: %s)", err, text)
	}

	return &result, nil
}

func buildJudgePrompt(response, idealResponse, criterion string) string {
	return fmt.Sprintf(`You are an AI response evaluator. Grade the following response against the criterion.

## Criterion
%s

## Ideal Response Description
%s

## Actual Response
%s

## Instructions
Evaluate whether the actual response meets the criterion. Return a JSON object with exactly these fields:
- "pass": boolean — true if the response meets the criterion
- "score": number between 0.0 and 1.0 — how well the criterion is met
- "reason": string — brief explanation of the grade

Return ONLY the JSON object, no other text.`, criterion, idealResponse, response)
}

// extractJSONFromMarkdown strips markdown code fences from LLM responses.
func extractJSONFromMarkdown(content string) string {
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json")
		if start >= 0 && start+7 < len(content) {
			after := content[start+7:]
			end := strings.Index(after, "```")
			if end > 0 {
				return strings.TrimSpace(after[:end])
			}
			return strings.TrimSpace(after)
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```")
		if start >= 0 && start+3 < len(content) {
			after := content[start+3:]
			end := strings.Index(after, "```")
			if end > 0 {
				return strings.TrimSpace(after[:end])
			}
			return strings.TrimSpace(after)
		}
	}
	return content
}

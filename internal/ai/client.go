package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"batchsync-ai/internal/model"
	"google.golang.org/genai"
)

// ConfigureGeminiClient creates a Gemini API client using the provided API key.
func ConfigureGeminiClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ai: API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: failed to create Gemini client: %w", err)
	}

	return client, nil
}

// Analyze sends a batch of LogEntries to Gemini and returns the structured results.
// Each entry in the batch gets its own ResultadoIA with the LogID preserved.
func Analyze(ctx context.Context, client *genai.Client, entries []model.LogEntry) ([]model.ResultadoIA, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	prompt := buildPrompt(entries)
	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(prompt), GetStructuredConfig())
	if err != nil {
		return nil, fmt.Errorf("ai: gemini request failed: %w", err)
	}

	text := result.Text()
	if text == "" {
		return nil, fmt.Errorf("ai: empty response from Gemini")
	}

	var raw struct {
		Analyze       string `json:"analyze"`
		SuggestedCode string `json:"suggested_code"`
		Criticality   int    `json:"criticality"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("ai: invalid JSON response: %w", err)
	}

	results := make([]model.ResultadoIA, len(entries))
	for i, entry := range entries {
		results[i] = model.ResultadoIA{
			LogID:         entry.ID,
			Analyze:       raw.Analyze,
			SuggestedCode: raw.SuggestedCode,
			Criticality:   raw.Criticality,
		}
	}

	return results, nil
}

// buildPrompt formats a batch of LogEntries into a single prompt for Gemini.
func buildPrompt(entries []model.LogEntry) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following log entries and provide a structured analysis:\n\n")
	for _, e := range entries {
		fmt.Fprintf(&sb, "[ID:%d] %s\n", e.ID, e.Message)
	}
	return sb.String()
}

// GetStructuredConfig returns a GenerateContentConfig that forces Gemini
// to respond with a JSON object matching the ResultadoIA schema.
func GetStructuredConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"analyze":        {Type: genai.TypeString},
				"suggested_code": {Type: genai.TypeString},
				"criticality":    {Type: genai.TypeInteger},
			},
			Required: []string{"analyze", "suggested_code", "criticality"},
		},
	}
}

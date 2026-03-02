package ai_test

import (
	"context"
	"os"
	"testing"

	"batchsync-ai/internal/ai"
	"batchsync-ai/internal/model"
)

func TestConfigureGeminiClient(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set — skipping integration test")
	}

	ctx := context.Background()
	_, err := ai.ConfigureGeminiClient(ctx, apiKey)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
}

func TestConfigureGeminiClient_EmptyKey(t *testing.T) {
	ctx := context.Background()
	_, err := ai.ConfigureGeminiClient(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}
}

func TestAnalyze_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set — skipping integration test")
	}

	ctx := context.Background()
	client, err := ai.ConfigureGeminiClient(ctx, apiKey)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	entries := []model.LogEntry{
		{ID: 1, Message: "ERROR: Connection timeout to database server after 30s at db.Connect() line 45"},
		{ID: 2, Message: "WARN: Disk usage at 92% on /dev/sda1"},
	}

	results, err := ai.Analyze(ctx, client, entries)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(results) != len(entries) {
		t.Fatalf("expected %d results, got %d", len(entries), len(results))
	}

	for i, r := range results {
		if r.LogID != entries[i].ID {
			t.Errorf("result[%d]: expected LogID %d, got %d", i, entries[i].ID, r.LogID)
		}
		if r.Analyze == "" {
			t.Errorf("result[%d]: analyze field is empty", i)
		}
	}
}

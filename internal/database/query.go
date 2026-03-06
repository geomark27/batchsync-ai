package database

import (
	"batchsync-ai/internal/model"
	"database/sql"
	"fmt"
	"strings"
)

func FetchPendingLogs(db *sql.DB, limit int) ([]model.LogEntry, error) {

	if limit <= 0 {
		return nil, fmt.Errorf("database: limit must be > 0")
	}

	query := `
		SELECT TOP (@p1) ID, Message
		FROM Logs
		WHERE ProcessedAt IS NULL
		ORDER BY ID ASC
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("database: query pending logs: %w", err)
	}

	defer rows.Close()

	entries := make([]model.LogEntry, 0, limit)
	for rows.Next() {
		var e model.LogEntry
		if err := rows.Scan(&e.ID, &e.Message); err != nil {
			return nil, fmt.Errorf("database: scan log entry: %w", err)
		}

		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: rows iteration error: %w", err)
	}

	return entries, nil
}

func markAsProcessedChunk(db *sql.DB, ids []int) error {
	placeHolders := make([]string, len(ids))
	args := make([]interface{}, len(ids))

	for i, id := range ids {
		placeHolders[i] = fmt.Sprintf("@p%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"UPDATE Logs SET ProcessedAt = GETUTCDATE() WHERE ID IN (%s)",
		strings.Join(placeHolders, ", "),
	)

	if _, err := db.Exec(query, args...); err != nil {
		return fmt.Errorf("database: mark as processd: %w", err)
	}

	return nil
}

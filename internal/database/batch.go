package database

import (
	"batchsync-ai/internal/config"
	"batchsync-ai/internal/model"
	"database/sql"
	"fmt"
	"strings"
)

func SaveBatchResult(db *sql.DB, results []model.ResultadoIA, cfg config.Config) error {

	if len(results) == 0 {
		return nil
	}

	maxRowsPerBatch := cfg.MPPQ / cfg.CPR

	for s := 0; s < len(results); s += maxRowsPerBatch {

		f := s + maxRowsPerBatch

		if f > len(results) {
			f = len(results)
		}

		if err := insertBatch(db, results[s:f], cfg); err != nil {
			return fmt.Errorf("database: batch [%d:%d]: %w", s, f, err)
		}
	}

	return nil
}

func insertBatch(db *sql.DB, results []model.ResultadoIA, cfg config.Config) error {
	cpr := cfg.CPR

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("database: begin transaction: %w", err)
	}

	defer tx.Rollback()

	placeholders := make([]string, 0, len(results))
	vals := make([]interface{}, 0, len(results)*cpr)

	for i, res := range results {
		n := i * cpr

		placeholders = append(placeholders,
			fmt.Sprintf("(@p%d, @p%d, @p%d, @p%d)", n+1, n+2, n+3, n+4),
		)
		vals = append(vals, res.LogID, res.Analyze, res.SuggestedCode, res.Criticality)
	}

	query := "INSERT INTO AnalisisLogs (LogID, Analisis, CodigoSugerido, Criticidad) VALUES " +
		strings.Join(placeholders, ", ")

	if _, err = tx.Exec(query, vals...); err != nil {
		return fmt.Errorf("database: exec batch insert: %w", err)
	}

	return tx.Commit()
}

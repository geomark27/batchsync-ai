# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Build the main binary (cmd/batchsync/main.go does not exist yet — Sprint 6)
go build -o batchsync ./cmd/batchsync

# Run all tests with race detector (required — all tests must pass with -race)
go test -race ./...

# Run a single package's tests
go test -race ./internal/processor/...

# Run a single test by name
go test -race -run TestConfigureGeminiClient ./internal/ai/...

# Lint
go vet ./...

# Clean up unused/indirect dependencies (run after any go get or removal)
go mod tidy

# Add a dependency
go get <package>
```

## Environment Setup

Copy `.env.example` to `.env` and fill in the required values before running:

```bash
SQLSERVER_CONN_STRING="sqlserver://user:password@host:1433?database=mydb"  # required
GEMINI_API_KEY="your-api-key-here"                                          # required
```

Optional env vars with defaults: `DB_MAX_OPEN_CONNS` (50), `DB_MAX_IDLE_CONNS` (10), `DB_CONN_MAX_LIFETIME` (30m), `BATCH_SIZE` (20), `MAX_WORKERS` (50). Duration format uses Go syntax (e.g. `30m`, `1h`).

The system performs a fail-fast validation at startup: missing `SQLSERVER_CONN_STRING` or `GEMINI_API_KEY` exits immediately with error. `InitDB` also calls `Ping()` and refuses to start if the database is unreachable.

## Architecture

BatchSync AI is a batch-processing backend — no HTTP endpoints. It reads records from SQL Server, enriches them with AI analysis via Google Gemini, and writes structured results back to SQL Server. Triggered by a scheduled job or CLI invocation.

### Pipeline

```
SQL Server → Extract N records
           → Chunk into blocks (BATCH_SIZE records each)
           → Goroutine pool (MAX_WORKERS semaphore) + rate limiter → Gemini API (structured JSON)
           → Collect results via channel
           → Batch INSERT to SQL Server (≤700 rows/batch)
```

### Package Structure

```
internal/
├── config/      config.Load() — reads/validates env vars, returns *Config
├── database/    InitDB(*Config) — connection pool; GuardarResultadosBatch (NOT YET BUILT — Sprint 4)
├── ai/          ConfigureGeminiClient, Analyze, GetStructuredConfig — Gemini integration
├── processor/   Run (orchestrator), chunk, procesarConGoroutines — goroutine pool
└── model/       Shared structs: LogEntry, ResultadoIA
cmd/batchsync/   main.go entrypoint (NOT YET BUILT — Sprint 6)
```

### Primary Integration Point

`processor.Run()` is the public entry point for the pipeline once a client and entries are available:

```go
results, err := processor.Run(ctx, geminiClient, entries, processor.Config{
    BatchSize:  cfg.BatchSize,  // entries per Gemini request (default: 20)
    MaxWorkers: cfg.MaxWorkers, // concurrent goroutines (default: 50)
    RateLimit:  0,              // req/sec; 0 defaults to MaxWorkers
})
```

### What Is NOT Yet Built

- **`internal/database/batch.go`** — `GuardarResultadosBatch(db, []ResultadoIA)` and `insertarBatch()` (Sprint 4)
- **`internal/database/db.go`** — `ObtenerLogsPendientes(db)` to read source records (Sprint 4)
- **`cmd/batchsync/main.go`** — ties config → InitDB → ConfigureGeminiClient → Run → GuardarResultadosBatch (Sprint 6)

### Critical Constraints

- **SQL Server parameter limit:** The `go-mssqldb` driver caps at 2,100 parameters per query. With 3 columns per row, that's a hard limit of **700 rows per INSERT**. `GuardarResultadosBatch` must chunk slices accordingly.
- **SQL Server placeholders:** Use positional `@p1, @p2, @p3...` syntax — not `?` (MySQL/SQLite style).
- **Batch atomicity:** Each INSERT batch (≤700 rows) is its own transaction. A failed batch rolls back only itself — previous successfully committed batches are not affected.
- **Gemini structured output:** Responses must conform to the JSON schema `{ analisis: string, codigo_sugerido: string, nivel_criticidad: integer }`. Free-text responses are rejected. Schema is enforced via `GetStructuredConfig()` in `internal/ai/client.go`.
- **Gemini model:** Hardcoded as `gemini-2.0-flash` in `ai.Analyze()`.
- **Error propagation:** The goroutine pool uses fail-fast semantics — the first worker error cancels all remaining work via context cancellation.

### Destination Table DDL

```sql
CREATE TABLE AnalisisLogs (
    LogID      INT           NOT NULL,
    Analisis   NVARCHAR(MAX) NOT NULL,
    Criticidad INT           NOT NULL
);
```

### Dependency Notes

- `golang.org/x/time` is used directly in `processor/pool.go` but listed as indirect in `go.mod`. Run `go mod tidy` to promote it to a direct dependency.
- Run `go mod tidy` after any `go get` or removal to keep `go.mod` and `go.sum` consistent.

See `docs/SPRINT_PLANNING.md` for task-level sprint status and `docs/TECHNICAL.md` for the full module specification.

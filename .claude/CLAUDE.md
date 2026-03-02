# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Build the main binary
go build -o batchsync ./cmd/batchsync

# Run all tests with race detector (required — all tests must pass with -race)
go test -race ./...

# Run a single package's tests
go test -race ./internal/processor/...

# Lint
go vet ./...

# Clean up unused/indirect dependencies
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

BatchSync AI is a batch-processing backend — no HTTP endpoints. It reads records from SQL Server, enriches them with AI analysis via Google Gemini, and writes structured results back to SQL Server. The intended trigger is a scheduled job or CLI invocation.

### Pipeline ("IA-Driven Batching")

```
SQL Server → Extract N records
           → Chunk into blocks (BATCH_SIZE records each)
           → Goroutine pool (MAX_WORKERS semaphore) → Gemini API (structured JSON output)
           → Collect results via channel
           → Batch INSERT to SQL Server (≤700 rows/batch)
```

### Package Structure

```
internal/
├── config/      config.Load() — reads/validates env vars, returns *Config
├── database/    InitDB(*Config) — connection pool setup; GuardarResultadosBatch (Sprint 4)
├── ai/          ConfigureGeminiClient, GetStructuredConfig — Gemini client + JSON schema (Sprint 2)
├── processor/   procesarConGoroutines — semaphore-based goroutine pool (Sprint 3)
└── model/       Shared structs: LogEntry, ResultadoIA
cmd/batchsync/   main.go entrypoint (Sprint 6)
```

### Critical Constraints

- **SQL Server parameter limit:** The `go-mssqldb` driver caps at 2,100 parameters per query. With 3 columns per row, that's a hard limit of **700 rows per INSERT**. `GuardarResultadosBatch` must chunk slices accordingly.
- **SQL Server placeholders:** Use positional `@p1, @p2, @p3...` syntax — not `?` (MySQL/SQLite style).
- **Batch atomicity:** Each INSERT batch (`≤700` rows) is its own transaction. A failed batch rolls back only itself — previous successfully committed batches are not affected.
- **Gemini structured output:** Responses must conform to the JSON schema `{ analisis: string, codigo_sugerido: string, nivel_criticidad: integer }`. Free-text responses are rejected.
- **Gemini concurrency:** The semaphore in `procesarConGoroutines` caps concurrent goroutines at `MAX_WORKERS`. A `golang.org/x/time/rate` rate limiter is also planned (Sprint 3) to respect API quotas.
- **Error propagation:** The goroutine pool uses fail-fast semantics — the first worker error is sent to `errCh` and the entire batch is aborted.

### Destination Table DDL

```sql
CREATE TABLE AnalisisLogs (
    LogID      INT           NOT NULL,
    Analisis   NVARCHAR(MAX) NOT NULL,
    Criticidad INT           NOT NULL
);
```

### Dependency Notes

- `google.golang.org/genai` is present in `go.mod` as an indirect dependency. When implementing Sprint 2, promote it to a direct dependency with `go get google.golang.org/genai`.
- `golang.org/x/time/rate` must be added explicitly for Sprint 3 rate limiting.
- Run `go mod tidy` after adding or removing dependencies to prune unused indirect entries.

See `docs/SPRINT_PLANNING.md` for task-level sprint status and `docs/TECHNICAL.md` for the full module specification.

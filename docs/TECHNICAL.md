# TECHNICAL.md — BatchSync AI

_Última actualización: 2026-03-02 (rev. Sprint 2)_

---

## Stack y Dependencias

| Componente | Tecnología | Versión |
| :---- | :---- | :---- |
| Lenguaje | Go | 1.24.0 |
| Base de datos | Microsoft SQL Server | <!-- TODO: verificar versión --> |
| Driver SQL Server | github.com/microsoft/go-mssqldb | v1.9.7 |
| SDK IA | google.golang.org/genai | v1.48.0 |
| Rate limiting | golang.org/x/time/rate | <!-- TODO: pendiente de agregar en Sprint 3 --> |
| Telemetría | go.opentelemetry.io/otel | <!-- TODO: pendiente de agregar en Sprint 6 --> |

> `go-mssqldb` v1.9.7 agregado en Sprint 1. `google.golang.org/genai` v1.48.0 promovido a dependencia directa en Sprint 2. Las dependencias de rate limiting y telemetría se agregarán en sprints subsiguientes.
> Dependencias indirectas activas: `golang.org/x/crypto` v0.45.0, `golang.org/x/text` v0.31.0, `github.com/shopspring/decimal` v1.4.0, `github.com/google/uuid` v1.6.0.

---

## Estructura de Módulos

```
batchsync-ai/
├── cmd/                    # Entrypoints (binarios compilables)
│   └── batchsync/          # main.go — punto de entrada principal
├── internal/               # Paquetes internos (no importables externamente)
│   ├── config/             # Lectura y validación de variables de entorno
│   ├── database/           # Pool de conexiones, queries, batch inserts
│   ├── ai/                 # Cliente Gemini, config structured outputs
│   ├── processor/          # Goroutine pool, chunking, orquestación del flujo
│   └── model/              # Structs compartidos: LogEntry, ResultadoIA, etc.
├── docs/                   # Documentación técnica y de negocio
│   ├── BatchSync AI.md     # Documento técnico core (arquitectura)
│   ├── BUSINESS_LOGIC.md   # Lógica de negocio
│   ├── TECHNICAL.md        # Este archivo
│   └── SPRINT_PLANNING.md  # Tracker de sprints
└── go.mod
```

> La estructura `cmd/` + `internal/` sigue la convención estándar de proyectos Go. Los paquetes en `internal/` solo pueden ser importados por código dentro de `batchsync-ai`.

---

## Módulos Internos

### `internal/config`

Lectura, validación y tipado de todas las variables de entorno del sistema.

**Funciones principales:**
- `Load() (*Config, error)` — Lee env vars, aplica defaults y valida las requeridas. Falla rápido si falta una variable obligatoria.

**Struct `Config`:**

```go
type Config struct {
    // SQL Server
    DBConnString      string        // SQLSERVER_CONN_STRING (requerida)
    DBMaxOpenConns    int           // DB_MAX_OPEN_CONNS    (default: 50)
    DBMaxIdleConns    int           // DB_MAX_IDLE_CONNS    (default: 10)
    DBConnMaxLifetime time.Duration // DB_CONN_MAX_LIFETIME (default: 30m)

    // Google Gemini
    GeminiAPIKey string // GEMINI_API_KEY (requerida)

    // Parámetros de procesamiento
    BatchSize  int // BATCH_SIZE   (default: 20)
    MaxWorkers int // MAX_WORKERS  (default: 50)
}
```

**Reglas de validación:**
- `SQLSERVER_CONN_STRING` y `GEMINI_API_KEY` son obligatorias; ausencia retorna error.
- `BATCH_SIZE` y `MAX_WORKERS` deben ser `> 0`; valores no parseables usan el default.

---

### `internal/database`

Responsable de toda interacción con SQL Server.

**Funciones principales:**
- `InitDB(cfg *config.Config) (*sql.DB, error)` — Inicializa el pool de conexiones con los valores de `cfg`. Ejecuta `Ping()` antes de retornar; si la BD no es alcanzable, cierra la conexión y retorna error.
- `GuardarResultadosBatch(db *sql.DB, resultados []ResultadoIA) error` — Divide en chunks de ≤700 filas e inserta en transacciones atómicas. <!-- TODO: implementar en Sprint 4 -->
- `insertarBatch(db *sql.DB, resultados []ResultadoIA) error` — Ejecuta un INSERT batch con placeholders `@p1..@pN` (sintaxis SQL Server). <!-- TODO: implementar en Sprint 4 -->

**Configuración del pool:**

| Parámetro | Valor | Razón |
| :---- | :---- | :---- |
| `MaxOpenConns` | 50 | Evita saturar SQL Server en picos |
| `MaxIdleConns` | 10 | Mantiene conexiones "calientes" |
| `ConnMaxLifetime` | 30 min | Recicla conexiones viejas / evita bloqueos de firewall |

**Tabla destino en SQL Server:**

```sql
CREATE TABLE AnalisisLogs (
    LogID      INT          NOT NULL,
    Analisis   NVARCHAR(MAX) NOT NULL,
    Criticidad INT          NOT NULL
);
-- TODO: agregar PK, índices, FK a tabla de logs fuente
```

---

### `internal/ai`

Responsable de la integración con la API de Google Gemini.

**Funciones principales:**
- `ConfigureGeminiClient(ctx context.Context, apiKey string) (*genai.Client, error)` — Crea el cliente Gemini con la API key proporcionada. Retorna error si `apiKey` está vacío.
- `GetStructuredConfig() *genai.GenerateContentConfig` — Retorna la config que fuerza respuesta JSON con schema estricto.

**Schema de respuesta forzada:**

```json
{
  "analyze": "string",
  "suggested_code": "string",
  "criticality": integer
}
```

> Los campos del schema usan nombres en inglés (alineados con los structs de `internal/model`). El modelo usado en pruebas de integración es `gemini-2.0-flash`.

**Variables de entorno requeridas:**

| Variable | Descripción |
| :---- | :---- |
| `GEMINI_API_KEY` | API key de Google AI Studio / Vertex AI |

---

### `internal/processor`

Orquesta el flujo completo: extracción → chunking → análisis paralelo → consolidación.

**Funciones principales:**
- `procesarConGoroutines(ctx, bloques, maxWorkers, fn)` — Goroutine pool con semáforo canal-based. Propaga el primer error de cualquier worker.

**Parámetros de concurrencia:**

| Parámetro | Valor recomendado | Descripción |
| :---- | :---- | :---- |
| `maxWorkers` | 50 | Goroutines concurrentes máximo |
| Tamaño de bloque | 20 registros | Registros por llamada a Gemini |

> El tamaño de bloque es configurable. Bloques más grandes reducen llamadas a la API pero aumentan tokens por request.

---

### `internal/model`

Structs compartidos entre paquetes.

```go
// LogEntry representa un registro fuente a analizar.
type LogEntry struct {
    ID      int
    Message string
    // TODO: agregar campos según esquema real de la BD fuente
}

// ResultadoIA es la salida estructurada de Gemini para un bloque.
type ResultadoIA struct {
    LogID         int
    Analyze       string
    SuggestedCode string
    Criticality   int
}
```

---

## Patrones de Arquitectura

**Pipeline Pattern** — El flujo de datos sigue etapas secuenciales con canales Go entre ellas (extracción → análisis → persistencia).

**Goroutine Pool / Worker Pool** — Semáforo implementado con canal bufferizado para controlar concurrencia hacia la API externa.

**Repository Pattern** — El paquete `database` abstrae las operaciones de persistencia, desacoplando la lógica de negocio del driver de SQL Server.

**Structured Output / Schema-First** — La integración con Gemini define el contrato de datos antes de la llamada, evitando parsing de texto libre.

---

## Endpoints

> Este proyecto es un servicio backend de procesamiento por lotes — **no expone endpoints HTTP** en su diseño base. El trigger de ejecución es interno (scheduled job, CLI, o llamada desde otro servicio).
>
> <!-- TODO: Si se agrega una API HTTP de control (ej. trigger manual, status check), documentar aquí. -->

---

## Configuración de Entorno

Ver `.env.example` en la raíz del proyecto para la referencia completa con comentarios.

```bash
# SQL Server
SQLSERVER_CONN_STRING="sqlserver://user:password@host:1433?database=mydb"

# Pool de conexiones (opcionales, tienen defaults)
DB_MAX_OPEN_CONNS=50        # default: 50
DB_MAX_IDLE_CONNS=10        # default: 10
DB_CONN_MAX_LIFETIME=30m    # default: 30m (formato Go: 30m, 1h, etc.)

# Google Gemini
GEMINI_API_KEY="your-api-key-here"

# Parámetros de procesamiento (opcionales, tienen defaults)
BATCH_SIZE=20           # registros por bloque enviado a Gemini (default: 20)
MAX_WORKERS=50          # goroutines concurrentes (default: 50)
```

---

## Despliegue

El binario Go es **estático y sin dependencias externas** en tiempo de ejecución, ideal para contenedores.

**Dockerfile mínimo:**
```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o batchsync ./cmd/batchsync

FROM scratch
COPY --from=builder /app/batchsync /batchsync
ENTRYPOINT ["/batchsync"]
```

**Plataformas de despliegue soportadas:**

| Plataforma | Servicio | Notas |
| :---- | :---- | :---- |
| AWS | ECS Fargate / Lambda | Lambda: revisar timeout para batches grandes |
| GCP | Cloud Run / Cloud Functions | Cloud Run preferido para batches de larga duración |

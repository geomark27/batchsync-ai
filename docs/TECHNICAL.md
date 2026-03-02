# TECHNICAL.md — BatchSync AI

_Última actualización: 2026-03-01 — Sprint 1 completado_

---

## Stack y Dependencias

| Componente | Tecnología | Versión |
| :---- | :---- | :---- |
| Lenguaje | Go | 1.25.4 |
| Base de datos | Microsoft SQL Server | <!-- TODO: verificar versión --> |
| Driver SQL Server | github.com/microsoft/go-mssqldb | v1.9.7 |
| SDK IA | google.golang.org/genai | <!-- TODO: verificar tras `go get` --> |
| Rate limiting | golang.org/x/time/rate | <!-- TODO: pendiente de agregar --> |
| Telemetría | go.opentelemetry.io/otel | <!-- TODO: pendiente de agregar --> |

> `go-mssqldb` v1.9.7 fue agregado en Sprint 1. Las dependencias de IA y telemetría se agregarán en sprints subsiguientes.

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

### `internal/database`

Responsable de toda interacción con SQL Server.

**Funciones principales:**
- `InitDB(connString string) (*sql.DB, error)` — Inicializa y configura el pool de conexiones.
- `GuardarResultadosBatch(db *sql.DB, resultados []ResultadoIA) error` — Divide en chunks de ≤700 filas e inserta en transacciones atómicas.
- `insertarBatch(db *sql.DB, resultados []ResultadoIA) error` — Ejecuta un INSERT batch con placeholders `@p1..@pN` (sintaxis SQL Server).

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
- `ConfigureGeminiClient(ctx context.Context) (*genai.Client, error)` — Crea el cliente usando `GEMINI_API_KEY` del entorno.
- `GetStructuredConfig() *genai.GenerateContentConfig` — Retorna la config que fuerza respuesta JSON con schema estricto.

**Schema de respuesta forzada:**

```json
{
  "analisis": "string",
  "codigo_sugerido": "string",
  "nivel_criticidad": integer
}
```

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
    Mensaje string
    // TODO: agregar campos según esquema real de la BD fuente
}

// ResultadoIA es la salida estructurada de Gemini para un bloque.
type ResultadoIA struct {
    LogID      int
    Analisis   string
    Criticidad int
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

```bash
# SQL Server
SQLSERVER_CONN_STRING="sqlserver://user:password@host:1433?database=mydb"

# Google Gemini
GEMINI_API_KEY="your-api-key-here"

# Parámetros de procesamiento (opcional, con defaults en código)
BATCH_SIZE=20           # registros por bloque enviado a Gemini
MAX_WORKERS=50          # goroutines concurrentes
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

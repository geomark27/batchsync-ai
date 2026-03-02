# **Documento Técnico: BatchSync AI (Integración de Gemini, Go y SQL Server)**

**Audiencia:** Arquitectos de Soluciones / Desarrolladores Backend

**Tecnologías:** Go (Golang), Microsoft SQL Server, Google GenAI SDK (Gemini)

**Objetivos Principales:** Paralelización de tareas, Reducción de I/O (Batching & Pooling), Integración de IA Generativa mediante salidas estructuradas.

---

## **1. Resumen Ejecutivo**

Este documento describe el patrón de arquitectura para integrar modelos de lenguaje grande (LLMs) de Google (Gemini) en aplicaciones backend de alto rendimiento escritas en Go, utilizando bases de datos relacionales (SQL Server) como almacenamiento persistente.

El diseño del proyecto **BatchSync AI** aprovecha las capacidades de concurrencia nativa de Go (Goroutines) y la gestión eficiente de conexiones para minimizar el "overhead" de red y las operaciones de entrada/salida (I/O) dentro de la sincronización de procesos de negocio por lotes.

---

## **2. Gestión de Infraestructura: Go + SQL Server**

Para evitar el agotamiento de puertos (SNAT port exhaustion, común en entornos cloud como AWS) y reducir la latencia, Go maneja las conexiones a SQL Server a través de un **Pool de Conexiones** integrado.

### **2.1 Configuración Óptima del Pool**

El objeto `*sql.DB` en Go no es una conexión, sino un manejador del pool. Su configuración correcta es vital para el rendimiento:

```go
package database

import (
    "database/sql"
    "time"
    _ "github.com/microsoft/go-mssqldb" // Driver de SQL Server
)

func InitDB(connString string) (*sql.DB, error) {
    db, err := sql.Open("sqlserver", connString)
    if err != nil {
        return nil, err
    }

    // REDUCCIÓN DE I/O Y OPTIMIZACIÓN DE RECURSOS:
    // 1. Limita las conexiones máximas para no saturar SQL Server.
    db.SetMaxOpenConns(50)

    // 2. Mantiene conexiones "calientes" para evitar el handshake TCP repetitivo.
    db.SetMaxIdleConns(10)

    // 3. Recicla conexiones viejas para evitar bloqueos de red o firewalls.
    db.SetConnMaxLifetime(30 * time.Minute)

    return db, nil
}
```

---

## **3. Integración de Inteligencia Artificial (Google Gemini)**

La integración con Gemini se realiza mediante el SDK nativo de Go (`google.golang.org/genai`). Para sistemas transaccionales, el texto libre de la IA es problemático. La solución es forzar a la IA a devolver **Salidas Estructuradas (Structured Outputs)** en formato JSON.

### **3.1 Inicialización y Esquema Estricto**

Definimos un esquema JSON para que Gemini analice, por ejemplo, métricas de rendimiento o consultas lentas, y devuelva datos predecibles.

```go
package ai

import (
    "context"
    "os"
    "google.golang.org/genai"
)

func ConfigureGeminiClient(ctx context.Context) (*genai.Client, error) {
    return genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  os.Getenv("GEMINI_API_KEY"),
        Backend: genai.BackendGoogleAI,
    })
}

// GetStructuredConfig obliga a Gemini a responder en JSON estructurado.
func GetStructuredConfig() *genai.GenerateContentConfig {
    return &genai.GenerateContentConfig{
        ResponseMIMEType: "application/json",
        ResponseSchema: &genai.Schema{
            Type: genai.TypeObject,
            Properties: map[string]*genai.Schema{
                "analisis":         {Type: genai.TypeString},
                "codigo_sugerido":  {Type: genai.TypeString},
                "nivel_criticidad": {Type: genai.TypeInteger},
            },
        },
    }
}
```

---

## **4. Patrón de Diseño Recomendado: "IA-Driven Batching"**

Este es el núcleo de la arquitectura, combinando paralelización y reducción de I/O.

**El Flujo de Trabajo:**

```
[SQL Server] → (1) Extracción 1,000 registros
                       ↓
              (2) Goroutine Pool (50 workers) con semáforo
                       ↓ (concurrente)
              (3) API Gemini → JSON estructurado por bloque
                       ↓ (canal de resultados)
              (4) Agrupación en memoria ([]ResultadoIA)
                       ↓
              (5) Batch INSERT único → [SQL Server]
```

1. **Extracción:** Go extrae 1,000 registros (ej. logs de errores) de SQL Server.
2. **Paralelización:** Go levanta hasta 50 *Goroutines* controladas por un semáforo. Cada una envía un bloque de errores a la API de Gemini simultáneamente.
3. **Agrupación en Memoria:** Las respuestas JSON de Gemini se reciben por un canal y se acumulan en un slice protegido.
4. **Batching a BD:** Se abre una **única transacción** hacia SQL Server y se insertan todos los resultados en un solo viaje de red (round-trip).

> **Límite importante de SQL Server:** El driver `go-mssqldb` tiene un máximo de **2,100 parámetros por query**. Con 3 columnas por fila, el límite es ~700 filas por batch. Para conjuntos mayores, dividir el slice en chunks antes de insertar.

### **4.1 Goroutine Pool con Control de Concurrencia (Paso 2)**

```go
package processor

import (
    "context"
    "sync"
)

// procesarConGoroutines ejecuta fn en paralelo con un máximo de maxWorkers goroutines.
func procesarConGoroutines(
    ctx context.Context,
    bloques [][]LogEntry,
    maxWorkers int,
    fn func([]LogEntry) (ResultadoIA, error),
) ([]ResultadoIA, error) {
    sem := make(chan struct{}, maxWorkers) // semáforo para limitar concurrencia
    resultsCh := make(chan ResultadoIA, len(bloques))
    errCh := make(chan error, 1)

    var wg sync.WaitGroup

    for _, bloque := range bloques {
        bloque := bloque // captura de variable para la goroutine
        wg.Add(1)
        sem <- struct{}{} // adquiere slot

        go func() {
            defer wg.Done()
            defer func() { <-sem }() // libera slot al terminar

            res, err := fn(bloque)
            if err != nil {
                select {
                case errCh <- err: // envía solo el primer error
                default:
                }
                return
            }
            resultsCh <- res
        }()
    }

    // Esperamos y cerramos canales
    go func() {
        wg.Wait()
        close(resultsCh)
    }()

    // Recolectamos resultados
    var resultados []ResultadoIA
    for res := range resultsCh {
        resultados = append(resultados, res)
    }

    // Verificamos errores
    select {
    case err := <-errCh:
        return nil, err
    default:
        return resultados, nil
    }
}
```

### **4.2 Inserción Batch en SQL Server (Paso 4)**

SQL Server con el driver `go-mssqldb` requiere placeholders posicionales `@p1, @p2, @p3...` — **no** el `?` de MySQL/SQLite.

```go
package database

import (
    "database/sql"
    "fmt"
    "strings"
)

type ResultadoIA struct {
    LogID      int
    Analisis   string
    Criticidad int
}

const maxParamsPerQuery = 2100 // límite de SQL Server
const colsPerRow = 3

// GuardarResultadosBatch inserta los resultados en batches respetando el límite de parámetros.
func GuardarResultadosBatch(db *sql.DB, resultados []ResultadoIA) error {
    maxRowsPerBatch := maxParamsPerQuery / colsPerRow // 700 filas por batch

    for inicio := 0; inicio < len(resultados); inicio += maxRowsPerBatch {
        fin := inicio + maxRowsPerBatch
        if fin > len(resultados) {
            fin = len(resultados)
        }
        if err := insertarBatch(db, resultados[inicio:fin]); err != nil {
            return fmt.Errorf("batch [%d:%d]: %w", inicio, fin, err)
        }
    }
    return nil
}

func insertarBatch(db *sql.DB, resultados []ResultadoIA) error {
    // 1. Iniciamos transacción (toma 1 conexión del pool)
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op si Commit() ya fue llamado

    // 2. Construimos la query con placeholders @p1, @p2, @p3... (sintaxis SQL Server)
    placeholders := make([]string, 0, len(resultados))
    vals := make([]interface{}, 0, len(resultados)*colsPerRow)

    for i, res := range resultados {
        n := i * colsPerRow
        placeholders = append(placeholders,
            fmt.Sprintf("(@p%d, @p%d, @p%d)", n+1, n+2, n+3),
        )
        vals = append(vals, res.LogID, res.Analisis, res.Criticidad)
    }

    query := "INSERT INTO AnalisisLogs (LogID, Analisis, Criticidad) VALUES " +
        strings.Join(placeholders, ", ")

    // 3. Ejecutamos un solo comando masivo (un solo viaje de red)
    if _, err = tx.Exec(query, vals...); err != nil {
        return err
    }

    // 4. Confirmamos transacción
    return tx.Commit()
}
```

---

## **5. Consideraciones de Arquitectura y Despliegue (AWS / GCP)**

| Componente | Ecosistema AWS | Ecosistema Google Cloud | Ventaja Arquitectónica en Go |
| :---- | :---- | :---- | :---- |
| **Cómputo** | AWS Lambda / ECS Fargate | Cloud Run / Cloud Functions | Binario estático, arranque en milisegundos, ideal para contenedores. |
| **Base de Datos** | Amazon RDS (SQL Server) | Cloud SQL (SQL Server) | El Pooling en Go evita saturar la BD de conexiones en picos de tráfico. |
| **IA / LLM** | Amazon Bedrock (Claude) | Vertex AI / AI Studio (Gemini) | Integración fluida vía SDK, ventana de contexto masiva (2M tokens) en Gemini. |

### **5.1 Rate Limiting de la API de Gemini**

Las 50 goroutines concurrentes pueden superar las cuotas de la API de Gemini (requests/minuto y tokens/minuto). El semáforo de la sección 4.1 controla la concurrencia en el cliente, pero se recomienda además un rate limiter basado en `time.Ticker` o una librería como `golang.org/x/time/rate` para respetar los límites de cuota.

---

## **6. Pruebas y Monitoreo**

### **6.1 Pruebas Unitarias y Race Detector**

Ejecutar siempre los tests con el flag `-race` para detectar condiciones de carrera en las goroutines:

```bash
go test -race ./...
```

Los tests deben cubrir:
- Que `procesarConGoroutines` no genere condiciones de carrera al acumular resultados.
- Que `GuardarResultadosBatch` divida correctamente en chunks cuando `len(resultados) > 700`.
- Que un error en una goroutine cancele el procesamiento sin deadlock.

### **6.2 Monitoreo con OpenTelemetry**

Implementar telemetría para medir:
- Tiempo de respuesta de la API de Gemini por bloque.
- Tiempo de inserción en disco de SQL Server por batch.
- Tasa de errores por goroutine.

```go
// Ejemplo de span para medir llamada a Gemini
ctx, span := tracer.Start(ctx, "gemini.analizar_bloque")
defer span.End()
```

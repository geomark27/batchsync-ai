# SPRINT_PLANNING.md — BatchSync AI

_Última actualización: 2026-03-03 (rev. Sprint 3)_

---

## Progreso General

| Sprint | Nombre | Estado | Avance |
| :---- | :---- | :---- | ----: |
| S1 | Infraestructura Base | ✅ Completo | 100% |
| S2 | Integración IA (Gemini) | ✅ Completo | 100% |
| S3 | Orquestación y Procesador | ✅ Completo | 100% |
| S4 | Persistencia y Batching | ⏳ Pendiente | 0% |
| S5 | Testing y Calidad | ⏳ Pendiente | 0% |
| S6 | Observabilidad y Despliegue | 🔄 En progreso | 20% |
| **Total** | | | **57%** |

---

## Sprint 1 — Infraestructura Base

**Estado:** ✅ Completo — **100%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 1.1 | Inicializar `go.mod` con nombre de módulo | ✅ |
| 1.2 | Crear estructura de directorios (`cmd/`, `internal/`) | ✅ |
| 1.3 | Implementar `internal/database/db.go` — `InitDB` con pool config | ✅ |
| 1.4 | Agregar dependencia `github.com/microsoft/go-mssqldb` v1.9.7 | ✅ |
| 1.5 | Implementar `internal/config/config.go` — lectura y validación de env vars | ✅ |

---

## Sprint 2 — Integración IA (Gemini)

**Estado:** ✅ Completo — **100%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 2.1 | Agregar dependencia `google.golang.org/genai` v1.48.0 | ✅ |
| 2.2 | Implementar `internal/ai/client.go` — `ConfigureGeminiClient(ctx, apiKey)` | ✅ |
| 2.3 | Implementar `GetStructuredConfig` con schema JSON (`analyze`, `suggested_code`, `criticality`) | ✅ |
| 2.4 | Definir `internal/model/types.go` — `LogEntry`, `ResultadoIA` | ✅ |
| 2.5 | Tests en `client_test.go`: validación key vacía + integración con `gemini-2.0-flash` | ✅ |

---

## Sprint 3 — Orquestación y Procesador

**Estado:** ✅ Completo — **100%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 3.1 | Implementar `internal/processor/chunker.go` — dividir `[]LogEntry` en bloques | ✅ |
| 3.2 | Implementar `internal/processor/pool.go` — `procesarConGoroutines` con semáforo | ✅ |
| 3.3 | Integrar rate limiter (`golang.org/x/time/rate`) para respetar cuota de Gemini | ✅ |
| 3.4 | Implementar propagación de errores desde goroutines (errCh, fail-fast) | ✅ |
| 3.5 | Implementar flujo orquestador completo en `internal/processor/orchestrator.go` | ✅ |

---

## Sprint 4 — Persistencia y Batching

**Estado:** ⏳ Pendiente — **0%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 4.1 | Implementar `internal/database/batch.go` — `insertarBatch` con placeholders `@p1..@pN` | ✅ |
| 4.2 | Implementar `GuardarResultadosBatch` con chunking automático (≤700 filas) | ✅ |
| 4.3 | Crear script DDL de tabla `AnalisisLogs` en SQL Server | ✅ |
| 4.4 | Implementar extracción de `LogEntries` pendientes desde SQL Server | ✅ |
| 4.5 | Implementar marcado de registros procesados (evitar reprocesamiento) | ✅ |

---

## Sprint 5 — Testing y Calidad

**Estado:** ⏳ Pendiente — **0%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 5.1 | Tests unitarios para `procesarConGoroutines` con `-race` flag | ❌ |
| 5.2 | Tests unitarios para `GuardarResultadosBatch` — chunking con >700 filas | ❌ |
| 5.3 | Test: goroutine con error no genera deadlock | ❌ |
| 5.4 | Test: placeholders `@p1..@pN` generados correctamente | ❌ |
| 5.5 | Test de integración end-to-end con BD de prueba | ❌ |

---

## Sprint 6 — Observabilidad y Despliegue

**Estado:** 🔄 En progreso — **20%**

| # | Tarea | Estado |
| :- | :---- | :----: |
| 6.1 | Implementar `cmd/batchsync/main.go` — entrypoint con flags/env | ❌ |
| 6.2 | Agregar OpenTelemetry: spans para llamadas a Gemini y SQL Server | ❌ |
| 6.3 | Crear `Dockerfile` multi-stage con `scratch` final | ❌ |
| 6.4 | Documentar variables de entorno en `.env.example` | ✅ |
| 6.5 | Configurar CI básico (lint + `go test -race ./...`) | ❌ |

---

## Tabla de Endpoints

> BatchSync AI no expone endpoints HTTP en su diseño base — es un servicio de procesamiento interno.

| Endpoint | Método | Descripción | Implementado |
| :---- | :---- | :---- | :----: |
| — | — | No aplica en diseño actual | — |

<!-- TODO: Si se agrega API HTTP de control (trigger, status), agregar filas aquí -->

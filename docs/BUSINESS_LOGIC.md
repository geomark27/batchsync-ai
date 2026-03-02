# BUSINESS_LOGIC.md — BatchSync AI

_Última actualización: 2026-03-01_

---

## Propósito del Sistema

BatchSync AI es una plataforma de procesamiento por lotes que automatiza el análisis de grandes volúmenes de registros de negocio utilizando inteligencia artificial generativa (Gemini). Su función central es enriquecer datos operativos crudos (ej. logs de errores, métricas de rendimiento, consultas lentas) con análisis, sugerencias de remediación y clasificación de criticidad, para luego persistir esos resultados de forma eficiente en la base de datos corporativa.

El sistema no expone una interfaz de usuario propia — es un componente backend que se integra en pipelines de procesamiento de datos existentes.

---

## Entidades del Dominio

**LogEntry** — Registro de entrada a procesar.
Representa un evento o métrica del sistema fuente (SQL Server). Puede ser un error de aplicación, una consulta lenta, un evento de negocio, etc. Es la materia prima del sistema.

**Bloque (Chunk)** — Agrupación de LogEntries para envío a la IA.
Un conjunto de N registros que se envía en una sola llamada a la API de Gemini. El tamaño del bloque determina el balance entre eficiencia de red y granularidad del análisis.

**ResultadoIA** — Salida estructurada del análisis de Gemini.
Contiene el análisis textual del bloque, código de remediación sugerido (si aplica), y un nivel de criticidad numérico. Esta entidad es el producto de valor del sistema.

**AnalisisLog** — Registro persistido en SQL Server.
La materialización de un `ResultadoIA` en la base de datos. Vincula el resultado del análisis con el `LogID` original para trazabilidad.

---

## Flujos de Negocio Clave

### Flujo Principal: Ciclo de Análisis por Lotes

```
Trigger (scheduled / on-demand)
    │
    ▼
[1] EXTRACCIÓN
    Leer N registros pendientes de análisis desde SQL Server
    → Se obtiene una lista de LogEntries sin procesar
    │
    ▼
[2] PARTICIÓN
    Dividir los registros en bloques de tamaño configurable
    → Se obtiene [][]LogEntry (ej. 1,000 registros → 20 bloques de 50)
    │
    ▼
[3] ANÁLISIS IA (PARALELO)
    Enviar cada bloque a Gemini concurrentemente (máx. 50 workers)
    → Gemini devuelve JSON: { analisis, codigo_sugerido, nivel_criticidad }
    → Control de cuota: rate limiter en cliente
    │
    ▼
[4] CONSOLIDACIÓN
    Recolectar ResultadoIA de todos los workers vía canal
    → Si cualquier worker falla, el ciclo retorna error (fail-fast)
    │
    ▼
[5] PERSISTENCIA
    Insertar todos los ResultadoIA en SQL Server en batches de ≤700 filas
    → Cada batch es una transacción atómica
    → Fallo en un batch no afecta batches previos ya confirmados
```

### Reglas de Negocio

**RN-01: Análisis Estructurado Obligatorio.**
Toda respuesta de Gemini debe conformarse al schema JSON definido. Respuestas en texto libre no son aceptadas. Si Gemini no puede generar JSON válido, el bloque se marca como fallido.

**RN-02: Trazabilidad por LogID.**
Cada `ResultadoIA` debe conservar el `LogID` del registro original. Sin esta vinculación, el resultado no tiene valor de negocio y no debe persistirse.

**RN-03: Atomicidad por Batch.**
Un batch de inserción es todo-o-nada. Si falla la inserción de cualquier fila dentro del batch, se hace rollback del batch completo. El sistema puede reintentar el batch fallido.

**RN-04: Límite de Parámetros SQL Server.**
No más de 700 filas por sentencia `INSERT` (límite de 2,100 parámetros con 3 columnas por fila). El sistema debe dividir automáticamente conjuntos mayores.

**RN-05: Control de Concurrencia hacia la API.**
No más de 50 goroutines concurrentes enviando peticiones a Gemini simultáneamente. Este límite protege tanto la cuota de la API como los recursos de red del host.

**RN-06: Criticidad como Señal de Prioridad.**
El campo `nivel_criticidad` (integer, inferido por Gemini) puede usarse por sistemas consumidores para priorizar remediaciones. El sistema no define el rango ni los umbrales — eso es responsabilidad del consumidor del dato.

---

## Casos de Uso

**CU-01: Análisis de logs de error de aplicación.**
El sistema extrae errores recientes, los agrupa en bloques, y Gemini proporciona diagnóstico y sugerencia de fix para cada grupo.

**CU-02: Análisis de consultas lentas de SQL Server.**
Se extraen query plans o textos de consultas lentas; Gemini sugiere índices, rewrites o cambios de arquitectura de datos.

**CU-03: Auditoría de eventos de negocio.**
Registros de transacciones o eventos son clasificados por Gemini según patrones de riesgo o anomalía.

<!-- TODO: verificar — los casos de uso concretos dependen del dominio del cliente final -->

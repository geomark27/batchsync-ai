-- ============================================================
-- BatchSync AI — DDL
-- Last updated: Sprint 4
-- ============================================================

-- ─── Source table: Logs ──────────────────────────────────────
-- Stores the raw operational records to be analyzed.
-- ProcessedAt = NULL indicates the record has not yet been processed.
CREATE TABLE Logs (
    ID          INT           NOT NULL IDENTITY(1,1) PRIMARY KEY,
    Message     NVARCHAR(MAX) NOT NULL,
    CreatedAt   DATETIME2     NOT NULL DEFAULT GETUTCDATE(),
    ProcessedAt DATETIME2     NULL
);

-- Filtered index to make FetchPendingLogs efficient.
CREATE INDEX IX_Logs_ProcessedAt ON Logs (ProcessedAt) WHERE ProcessedAt IS NULL;

-- ─── Destination table: AnalisisLogs ─────────────────────────
-- Persists the structured results produced by Gemini.
CREATE TABLE AnalisisLogs (
    ID            INT           NOT NULL IDENTITY(1,1) PRIMARY KEY,
    LogID         INT           NOT NULL,
    Analysis      NVARCHAR(MAX) NOT NULL,
    SuggestedCode NVARCHAR(MAX) NOT NULL,
    Criticality   INT           NOT NULL,
    CreatedAt     DATETIME2     NOT NULL DEFAULT GETUTCDATE(),

    CONSTRAINT FK_AnalisisLogs_Logs FOREIGN KEY (LogID) REFERENCES Logs(ID)
);

CREATE INDEX IX_AnalisisLogs_LogID       ON AnalisisLogs (LogID);
CREATE INDEX IX_AnalisisLogs_Criticality ON AnalisisLogs (Criticality);
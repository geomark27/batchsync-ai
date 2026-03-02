package model

type LogEntry struct {
    ID int
    Message string
}

type ResultadoIA struct {
    LogID         int
    Analyze       string
    SuggestedCode string
    Criticality   int
}
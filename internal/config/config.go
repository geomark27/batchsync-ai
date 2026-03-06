package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config agrupa todas las variables de entorno del sistema.
type Config struct {
	// SQL Server
	DBConnString      string
	DBMaxOpenConns    int           // default: 50
	DBMaxIdleConns    int           // default: 10
	DBConnMaxLifetime time.Duration // default: 30m

	// Google Gemini
	GeminiAPIKey string

	// Parámetros de procesamiento
	BatchSize  int // registros por bloque enviado a Gemini (default: 20)
	MaxWorkers int // goroutines concurrentes máximo (default: 50)

	MPPQ int // max parameters per query (go-mssqldb hard limit: 2100)
	CPR  int // columns per row inserted into AnalisisLogs
}

// Load lee las variables de entorno y valida las obligatorias.
// Retorna error si alguna variable requerida está ausente.
func Load() (*Config, error) {
	cfg := &Config{
		DBConnString:      os.Getenv("SQLSERVER_CONN_STRING"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 50),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		BatchSize:         getEnvInt("BATCH_SIZE", 20),
		MaxWorkers:        getEnvInt("MAX_WORKERS", 50),
		MPPQ:              getEnvInt("MAX_PARAMS_PER_QUERY", 2100),
		CPR:               getEnvInt("COLS_PER_ROW", 4),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DBConnString == "" {
		return fmt.Errorf("config: SQLSERVER_CONN_STRING es requerida")
	}
	if c.GeminiAPIKey == "" {
		return fmt.Errorf("config: GEMINI_API_KEY es requerida")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("config: BATCH_SIZE debe ser mayor a 0")
	}
	if c.MaxWorkers <= 0 {
		return fmt.Errorf("config: MAX_WORKERS debe ser mayor a 0")
	}
	if c.MPPQ <= 0 {
		return fmt.Errorf("config: MAX_PARAMS_PER_QUERY debe ser mayor a 0")
	}
	if c.CPR <= 0 {
		return fmt.Errorf("config: COLS_PER_ROW debe ser mayor a 0")
	}
	return nil
}

// getEnvInt lee una variable de entorno como entero.
// Retorna defaultVal si la variable no existe o no es un entero válido.
func getEnvInt(key string, defaultVal int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// getEnvDuration lee una variable de entorno como time.Duration (ej. "30m", "1h").
// Retorna defaultVal si la variable no existe o no es parseable.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return defaultVal
	}
	return d
}

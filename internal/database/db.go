package database

import (
	"database/sql"
	"fmt"

	"batchsync-ai/internal/config"

	_ "github.com/microsoft/go-mssqldb"
)

// InitDB abre la conexión a SQL Server y configura el pool con los valores de cfg.
// connString debe tener el formato:
//
//	sqlserver://user:password@host:1433?database=mydb
func InitDB(cfg *config.Config) (*sql.DB, error) {
	if cfg.DBConnString == "" {
		return nil, fmt.Errorf("database: conn string vacío")
	}

	db, err := sql.Open("sqlserver", cfg.DBConnString)
	if err != nil {
		return nil, fmt.Errorf("database: error abriendo conexión: %w", err)
	}

	// Verificamos que la BD sea alcanzable antes de retornar.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database: ping fallido: %w", err)
	}

	// Reducción de I/O y optimización de recursos (valores desde .env):
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)       // evita saturar SQL Server en picos
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)       // mantiene conexiones "calientes"
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime) // recicla conexiones viejas

	return db, nil
}

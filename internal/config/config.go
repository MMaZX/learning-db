// Package config carga la configuración del MCP desde variables de entorno / .env.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config contiene toda la configuración del servidor MCP.
type Config struct {
	ServerName string
	LogLevel   string

	MCPTransport string
	MCPHTTPAddr  string
	MCPAuthToken string

	DBHost     string
	DBPort     int
	DBDatabase string
	DBUsername string
	DBPassword string

	QueryTimeout         time.Duration
	MaxRows              int
	MaxColumns           int
	MaxResultSizeBytes   int64
	MaxConcurrentQueries int
	SlowQueryThreshold   time.Duration

	ExcludedTables  map[string]bool
	ExcludedColumns map[string]bool

	KnowledgePath string
	DataPath      string
}

// Load lee un archivo .env (si existe) y construye la configuración.
// No falla si el .env no existe: en producción las variables pueden venir del entorno.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		ServerName: getEnv("MCP_SERVER_NAME", "db-intelligence"),
		LogLevel:   getEnv("MCP_LOG_LEVEL", "info"),

		MCPTransport: strings.ToLower(getEnv("MCP_TRANSPORT", "stdio")),
		MCPHTTPAddr:  getEnv("MCP_HTTP_ADDR", "127.0.0.1:8080"),
		MCPAuthToken: getEnv("MCP_AUTH_TOKEN", ""),

		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBDatabase: getEnv("DB_DATABASE", ""),
		DBUsername: getEnv("DB_USERNAME", ""),
		DBPassword: getEnv("DB_PASSWORD", ""),

		KnowledgePath: getEnv("MCP_KNOWLEDGE_PATH", "./knowledge"),
		DataPath:      getEnv("MCP_DATA_PATH", "./data"),
	}

	var err error
	if cfg.DBPort, err = getEnvInt("DB_PORT", 3306); err != nil {
		return nil, err
	}
	if cfg.QueryTimeout, err = getEnvDuration("MCP_QUERY_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.MaxRows, err = getEnvInt("MCP_MAX_ROWS", 1000); err != nil {
		return nil, err
	}
	if cfg.MaxColumns, err = getEnvInt("MCP_MAX_COLUMNS", 100); err != nil {
		return nil, err
	}
	maxResultSize := getEnv("MCP_MAX_RESULT_SIZE", "5MB")
	if cfg.MaxResultSizeBytes, err = parseByteSize(maxResultSize); err != nil {
		return nil, fmt.Errorf("MCP_MAX_RESULT_SIZE inválido: %w", err)
	}
	if cfg.MaxConcurrentQueries, err = getEnvInt("MCP_MAX_CONCURRENT_QUERIES", 5); err != nil {
		return nil, err
	}
	if cfg.SlowQueryThreshold, err = getEnvDuration("MCP_SLOW_QUERY_THRESHOLD", 2*time.Second); err != nil {
		return nil, err
	}

	cfg.ExcludedTables = toSet(getEnv("MCP_EXCLUDED_TABLES", ""))
	cfg.ExcludedColumns = toSet(getEnv("MCP_EXCLUDED_COLUMNS", "password,token,secret,pass,contrasena,contraseña"))

	if cfg.DBDatabase == "" || cfg.DBUsername == "" {
		return nil, fmt.Errorf("DB_DATABASE y DB_USERNAME son obligatorios (configúralos en .env o el entorno)")
	}

	switch cfg.MCPTransport {
	case "stdio":
		// ok
	case "http":
		if cfg.MCPAuthToken == "" {
			return nil, fmt.Errorf("MCP_AUTH_TOKEN es obligatorio cuando MCP_TRANSPORT=http")
		}
	default:
		return nil, fmt.Errorf("MCP_TRANSPORT inválido (%q): debe ser \"stdio\" o \"http\"", cfg.MCPTransport)
	}

	return cfg, nil
}

// DSN construye el Data Source Name para el driver MySQL.
// multiStatements queda deshabilitado deliberadamente: es una capa extra de
// defensa contra SQL apilado, independiente del validador de la capa security.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=false&interpolateParams=false&timeout=5s&readTimeout=%s",
		c.DBUsername, c.DBPassword, c.DBHost, c.DBPort, c.DBDatabase, c.QueryTimeout.String(),
	)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s debe ser un entero: %w", key, err)
	}
	return n, nil
}

func getEnvDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s debe ser una duración válida (ej. 10s): %w", key, err)
	}
	return d, nil
}

func parseByteSize(v string) (int64, error) {
	v = strings.TrimSpace(strings.ToUpper(v))
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(v, "KB"):
		multiplier = 1024
		v = strings.TrimSuffix(v, "KB")
	case strings.HasSuffix(v, "MB"):
		multiplier = 1024 * 1024
		v = strings.TrimSuffix(v, "MB")
	case strings.HasSuffix(v, "GB"):
		multiplier = 1024 * 1024 * 1024
		v = strings.TrimSuffix(v, "GB")
	case strings.HasSuffix(v, "B"):
		v = strings.TrimSuffix(v, "B")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}

func toSet(csv string) map[string]bool {
	set := make(map[string]bool)
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			set[item] = true
		}
	}
	return set
}

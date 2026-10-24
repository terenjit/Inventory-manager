package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBDSN      string
	DBTestDSN  string
	JWTSecret  string
	AppPort    string
	CORSOrigin string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		DBDSN:      getEnv("DB_DSN", "root:secret@tcp(localhost:3306)/textile_pos?parseTime=true&charset=utf8mb4"),
		DBTestDSN:  getEnv("DB_TEST_DSN", "root:secret@tcp(localhost:3307)/textile_pos_test?parseTime=true&charset=utf8mb4"),
		JWTSecret:  getEnv("JWT_SECRET", "dev-secret"),
		AppPort:    getEnv("APP_PORT", "8080"),
		CORSOrigin: getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
}

func LoadConfig() *Config {
	// Load .env file if it exists (for local development)
	_ = godotenv.Load()

	config := &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/queueflow?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		Port:        getEnv("PORT", "8080"),
	}

	if config.JWTSecret == "your-secret-key-change-in-production" {
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET environment variable in production!")
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	CircleAPIKey string
	Env         string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		CircleAPIKey: getEnv("CIRCLE_API_KEY", ""),
		Env:          getEnv("ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

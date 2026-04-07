package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr    string
	PaymentAddr   string
	DatabaseURL   string
	PaymentDBURL  string
	SessionSecret string
	CSRFSecret    string
	PaymentURL    string
	PaymentSecret string
	Env           string
	UploadsDir    string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	return &Config{
		ServerAddr:    getEnv("SERVER_ADDR", ":8080"),
		PaymentAddr:   getEnv("PAYMENT_ADDR", ":8081"),
		DatabaseURL:   mustEnv("DATABASE_URL"),
		PaymentDBURL:  getEnv("PAYMENT_DB_URL", getEnv("DATABASE_URL", "")),
		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production-32chars!"),
		CSRFSecret:    getEnv("CSRF_SECRET", "change-me-to-32-byte-csrf-secret!"),
		PaymentURL:    getEnv("PAYMENT_URL", "http://localhost:8081"),
		PaymentSecret: getEnv("PAYMENT_SECRET", "shared-secret-between-services"),
		Env:           getEnv("ENV", "development"),
		UploadsDir:    getEnv("UPLOADS_DIR", "uploads"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func (c *Config) IsDev() bool {
	return c.Env == "development"
}

package config

import (
	"cmp"
	"os"

	"github.com/joho/godotenv"
)

// AppConfig contains configuration required by the HTTP service.
type AppConfig struct {
	Port        string
	ServiceName string
	// ServiceToken guards /internal routes (X-Service-Token).
	ServiceToken string
}

// LoadConfig loads an optional .env file and returns environment-backed settings.
func LoadConfig() AppConfig {
	_ = godotenv.Load()

	return AppConfig{
		Port:         cmp.Or(os.Getenv("PORT"), "5013"),
		ServiceName:  cmp.Or(os.Getenv("SERVICE_NAME"), "asset-transaction-service"),
		ServiceToken: os.Getenv("SERVICE_TOKEN"),
	}
}

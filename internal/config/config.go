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
	// PostgresDB is the database holding the tenant schemas for this service.
	PostgresDB string
	// KafkaBrokers is the comma-separated Redpanda broker list for the iam.user.renamed consumer.
	KafkaBrokers string
}

// LoadConfig loads an optional .env file and returns environment-backed settings.
func LoadConfig() AppConfig {
	_ = godotenv.Load()

	return AppConfig{
		Port:         cmp.Or(os.Getenv("PORT"), "5013"),
		ServiceName:  cmp.Or(os.Getenv("SERVICE_NAME"), "asset-transaction-service"),
		ServiceToken: os.Getenv("SERVICE_TOKEN"),
		PostgresDB:   cmp.Or(os.Getenv("POSTGRES_DB"), "tagsamurai_asset_transaction"),
		KafkaBrokers: cmp.Or(os.Getenv("KAFKA_BROKERS"), "localhost:9092"),
	}
}

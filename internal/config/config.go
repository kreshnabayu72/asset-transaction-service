package config

import (
	"cmp"
	"os"
	"strconv"

	coreConfig "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/config"
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
	// IAMBaseURL is iam's internal address; the RBAC middleware asks it for the caller's roles.
	IAMBaseURL string
	// EnterpriseManagementBaseURL is the internal EMS address for category and brand reads.
	EnterpriseManagementBaseURL string
	// MasterAssetRegistryBaseURL is master-asset-registry's internal address for the category, asset and asset-name lookups.
	MasterAssetRegistryBaseURL string
	// ServiceCenterSnowflakeNodeID is deployment-unique; nil leaves ID-generating writes fail-closed.
	ServiceCenterSnowflakeNodeID *int64
	// TemporalHost and TemporalPort identify Temporal for post-commit service workflows.
	TemporalHost      string
	TemporalPort      string
	TemporalNamespace string
	// AnalyticsReportingTaskQueue is the queue that owns RecordChangeLogsWorkflow.
	AnalyticsReportingTaskQueue string
	// TemporalTaskQueue is this service worker queue for tenant migration activities.
	TemporalTaskQueue string
}

// LoadConfig loads an optional .env file and returns environment-backed settings.
func LoadConfig() AppConfig {
	_ = godotenv.Load()

	var serviceCenterSnowflakeNodeID *int64
	if nodeID, err := strconv.ParseInt(os.Getenv("SERVICE_CENTER_SNOWFLAKE_NODE_ID"), 10, 64); err == nil {
		serviceCenterSnowflakeNodeID = &nodeID
	}

	return AppConfig{
		Port:                         cmp.Or(os.Getenv("PORT"), "5013"),
		ServiceName:                  cmp.Or(os.Getenv("SERVICE_NAME"), "asset-transaction-service"),
		ServiceToken:                 os.Getenv("SERVICE_TOKEN"),
		PostgresDB:                   cmp.Or(os.Getenv("POSTGRES_DB"), "tagsamurai_asset_transaction"),
		KafkaBrokers:                 cmp.Or(os.Getenv("KAFKA_BROKERS"), "localhost:9092"),
		IAMBaseURL:                   os.Getenv("IAM_BASE_URL"),
		EnterpriseManagementBaseURL:  cmp.Or(os.Getenv("ENTERPRISE_MANAGEMENT_BASE_URL"), "http://localhost:8090"),
		MasterAssetRegistryBaseURL:   os.Getenv("MASTER_ASSET_REGISTRY_BASE_URL"),
		ServiceCenterSnowflakeNodeID: serviceCenterSnowflakeNodeID,
		TemporalHost:                 os.Getenv("TEMPORAL_HOST"),
		TemporalPort:                 cmp.Or(os.Getenv("TEMPORAL_PORT"), "7233"),
		TemporalNamespace:            cmp.Or(os.Getenv("TEMPORAL_NAMESPACE"), "default"),
		AnalyticsReportingTaskQueue:  cmp.Or(os.Getenv("ANALYTICS_REPORTING_TASK_QUEUE"), "analytics-reporting-service"),
		TemporalTaskQueue:            cmp.Or(os.Getenv("TEMPORAL_TASK_QUEUE"), "asset-transaction-service"),
	}
}

// PostgresDSN returns the connection string shared by PostgreSQL and tenant migration clients.
func (c AppConfig) PostgresDSN() string {
	return coreConfig.PostgresDSN(c.PostgresDB)
}

// Package repository reads the transaction settings storage of one tenant schema.
package repository

import (
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

// ServicerFactory opens the PostgreSQL servicer of one tenant (clientID); the caller closes it.
type ServicerFactory func(clientID string) coreService.PostgreSQLServicer

// TransactionSettingsRepository reads transaction_rule_fields, transaction_rule_overrides and
// transaction_settings.settings_version.
type TransactionSettingsRepository struct {
	open ServicerFactory
}

// NewTransactionSettingsRepository builds the repository over a per-tenant servicer factory.
func NewTransactionSettingsRepository(open ServicerFactory) *TransactionSettingsRepository {
	return &TransactionSettingsRepository{open: open}
}

// NewTenantServicerFactory opens the servicer of the tenant schema of clientID in database dbName.
func NewTenantServicerFactory(dbName string) ServicerFactory {
	return func(clientID string) coreService.PostgreSQLServicer {
		return coreService.MakePostgreSQLService(clientID + ":" + dbName)
	}
}

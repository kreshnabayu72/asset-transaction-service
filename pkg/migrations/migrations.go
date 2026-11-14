// Package migrations owns this service's SQL files and nothing else. The migration logic
// lives in mig; here we only say which files exist and expose the operations Temporal
// needs to reach by name.
//
// Every table of this service is tenant-scoped, so there is no public migration set.
package migrations

import (
	"context"
	"embed"

	"gitea.tagsamurai.local/Wangsit-Developer/mig/pkg/runner"
)

// SQL is embedded so the binary is self-contained: no dependency on the working directory.
//
//go:embed tenant/*.sql
var tenantFS embed.FS

// Tenant migrates a tenant's own schema.
func Tenant(connStr string) (*runner.Runner, error) {
	return runner.NewSub(connStr, tenantFS, "tenant")
}

// Activities provisions this service's tenant schemas. It is registered with Temporal, so the
// client-creation saga in enterprise-management reaches it by name, and boot calls it directly
// to catch up on every tenant that already exists.
type Activities struct {
	tenant *runner.Runner
}

func NewActivities(tenant *runner.Runner) *Activities {
	return &Activities{tenant: tenant}
}

// MigrateTenant provisions the named tenants' schemas and brings them up to date. Idempotent.
func (a *Activities) MigrateTenant(ctx context.Context, clientIDs []string) error {
	return a.tenant.UpSchemas(ctx, clientIDs)
}

// MigrateExisting brings up to date only the schemas that already exist here: the fallback
// for a boot that cannot reach the tenant list.
func (a *Activities) MigrateExisting(ctx context.Context) error {
	return a.tenant.UpAllSchemas(ctx)
}

// Verify round-trips the migrations on a throwaway schema, so broken SQL is caught before any
// tenant is touched.
func (a *Activities) Verify(ctx context.Context) error {
	return a.tenant.Verify(ctx)
}

// DropTenantSchema removes a tenant's schema and everything in it. It is the compensation
// for creating a new client, and the dev client-delete step.
func (a *Activities) DropTenantSchema(ctx context.Context, clientID string) error {
	return a.tenant.DropSchema(ctx, clientID)
}

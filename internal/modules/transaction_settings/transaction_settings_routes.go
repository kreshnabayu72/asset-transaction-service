// Package transaction_settings mounts the Fixed Asset Transaction Settings API.
package transaction_settings

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/handler"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/rbac"
	"github.com/gofiber/fiber/v3"
)

// SetupRoutes mounts GET and PATCH /transaction-settings, POST /transaction-settings/export,
// POST /transaction-settings/effective-rules and the override routes (POST /overrides, POST
// /overrides/list, POST /overrides/options, DELETE /overrides).
// Both read routes pass for See All (Total Control, Default User, Read Only) and the View verb of
// Manage transaction settings or Manage asset & TAG; every other role gets 403. Mount after
// LoadUserRoles (ADR 013).
func SetupRoutes(app *fiber.App, h *handler.Handler) {
	canRead := rbac.Require(rbac.Any(
		rbac.FixedAssetSeeAll(),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyManageTransactionSettings),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyManageAssetAndTAG),
	))
	// Save: Total Control, Default User and Manage transaction settings (Update); Read Only,
	// Manage asset & TAG and other roles get 403. Rechecked on every request.
	canSave := rbac.Require(rbac.Any(
		rbac.FixedAssetTotalControl(),
		rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageTransactionSettings),
	))
	// Override create and delete: Total Control, Default User, Manage transaction settings (Update)
	// or Manage asset & TAG (Update); Read Only access level answers 403 even with an Update role.
	// The target scope is checked per row by the service.
	canWriteOverrides := rbac.Require(
		notReadOnly,
		rbac.Any(
			rbac.FixedAssetTotalControl(),
			rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageTransactionSettings),
			rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG),
		),
	)
	group := app.Group("/fixed-asset/transaction-settings")
	group.Get("/", canRead, h.ReadTransactionSettings)
	group.Patch("/", canSave, h.SaveTransactionSettings)
	group.Post("/export", canRead, h.ExportTransactionSettings)
	group.Post("/overrides/list", canRead, h.ListOverrides)
	group.Post("/overrides/options", canRead, h.OverridesOptions)
	group.Post("/overrides", canWriteOverrides, h.CreateOverrides)
	group.Delete("/overrides", canWriteOverrides, h.DeleteOverrides)
	// Effective rules: any authenticated user of the entity (SD-89); no settings capability needed.
	group.Post("/effective-rules", h.ResolveEffectiveRules)
}

// notReadOnly rejects the Read Only access level: it limits every right to Read, so an Update
// role held next to it does not allow a write.
func notReadOnly(_ fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	return !roles.IsReadOnly(enum.ModuleAccessFixedAsset), nil
}

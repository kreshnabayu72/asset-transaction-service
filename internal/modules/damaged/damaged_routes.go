// Package damaged mounts the Fixed Asset Damaged read routes.
package damaged

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/handler"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/rbac"
	"github.com/gofiber/fiber/v3"
)

// SetupRoutes mounts GET /v2/asset-transaction/damaged/summary and
// GET /v2/asset-transaction/damaged/assets/:assetId/running-record. Mount after
// tsMiddleware.LoadUserRoles. The summary requires a Damaged view scope (Manage asset & TAG Read or
// Update, or View asset value); the running record is gated per asset in the service: group scope
// or the caller holding the asset.
func SetupRoutes(app *fiber.App, h *handler.Handler) {
	canViewDamaged := rbac.Require(rbac.Any(
		rbac.FixedAssetSeeAll(),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyManageAssetAndTAG),
		rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyViewAssetValue),
	))
	group := app.Group("/v2/asset-transaction/damaged")
	group.Get("/summary", canViewDamaged, h.GetSummary)
	group.Get("/assets/:assetId/running-record", h.GetRunningRecord)
}

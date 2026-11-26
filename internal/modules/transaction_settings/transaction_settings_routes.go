// Package transaction_settings mounts the Fixed Asset Transaction Settings API.
package transaction_settings

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/handler"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/rbac"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	"github.com/gofiber/fiber/v3"
)

// SetupRoutes mounts GET and PATCH /transaction-settings, POST /transaction-settings/export,
// POST /transaction-settings/effective-rules and the override routes (POST /overrides, POST
// /overrides/list, POST /overrides/options, DELETE /overrides).
// Both read routes pass for See All (Total Control, Default User, Read Only) and the View verb of
// Manage transaction settings or Manage asset & TAG; every other role gets 403. Mount after
// LoadUserRoles (ADR 013).
func SetupRoutes(app *fiber.App, h *handler.Handler) {
	canRead := capability(rbac.Any(
		rbac.FixedAssetSeeAll(),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyManageTransactionSettings),
		rbac.FixedAssetSystemRole(middleware.PermView, enum.SystemRoleKeyManageAssetAndTAG),
	))
	// Save: Total Control, Default User and Manage transaction settings (Update); Read Only,
	// Manage asset & TAG and other roles get 403. Rechecked on every request.
	canSave := capability(rbac.Any(
		rbac.FixedAssetTotalControl(),
		rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageTransactionSettings),
	))
	// Override create and delete: Total Control, Default User, Manage transaction settings (Update)
	// or Manage asset & TAG (Update); Read Only access level answers 403 even with an Update role.
	// The target scope is checked per row by the service.
	canWriteOverrides := capability(
		notReadOnly,
		rbac.Any(
			rbac.FixedAssetTotalControl(),
			rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageTransactionSettings),
			rbac.FixedAssetSystemRole(middleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG),
		),
	)
	// Every route first refuses a network actor (SD-4 entity_type; LLD Common Invariants).
	clientOnly := clientEntity(h)
	// Writes also refuse a Suspended or Expired client before the capability check (SD-5, SD-31).
	writable := clientWritable(h)
	group := app.Group("/fixed-asset/transaction-settings")
	group.Get("/", clientOnly, canRead, h.ReadTransactionSettings)
	group.Patch("/", clientOnly, writable, canSave, h.SaveTransactionSettings)
	group.Post("/export", clientOnly, canRead, h.ExportTransactionSettings)
	group.Post("/overrides/list", clientOnly, canRead, h.ListOverrides)
	group.Post("/overrides/options", clientOnly, canRead, h.OverridesOptions)
	group.Post("/overrides", clientOnly, writable, canWriteOverrides, h.CreateOverrides)
	group.Delete("/overrides", clientOnly, writable, canWriteOverrides, h.DeleteOverrides)
	// Effective rules: any authenticated user of the entity (SD-89); no settings capability needed.
	group.Post("/effective-rules", clientOnly, h.ResolveEffectiveRules)
}

// reasonCapability is the SD-4 data.reason of a capability denial.
const reasonCapability = "capability"

// capability is rbac.Require with the SD-4/SD-31 denial body: 403 {"message":"Forbidden",
// "data":{"reason":"capability"}} instead of the shared gate's error PERMISSION_DENIED. Rules are
// ANDed like rbac.Require; a missing role snapshot answers 401 and a rule error its own status.
func capability(rules ...rbac.Rule) fiber.Handler {
	return func(c fiber.Ctx) error {
		roles, ok := middleware.GetUserRoles(c)
		if !ok {
			return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
		}
		for _, rule := range rules {
			passed, err := rule(c, roles)
			if err != nil {
				return coreEntity.ToHttpError(err).SendResponse(c)
			}
			if !passed {
				denied := coreEntity.Forbidden("Forbidden")
				denied.Data = fiber.Map{"reason": reasonCapability}
				return denied.SendResponse(c)
			}
		}
		return c.Next()
	}
}

// reasonEntityType is the SD-4 data.reason of a network actor (a Principal, Distributor or Partner entity).
const reasonEntityType = "entity_type"
const reasonClientSuspended = "client_suspended_or_expired"

// clientEntity answers 403 data.reason "entity_type" for a network actor and lets a Client tenant through. A failed
// lookup, or an entity type outside the four, answers 500.
func clientEntity(h *handler.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		entityType, err := h.EntityType(c.Context(), c.Get(constant.HEADER_CLIENT_ID))
		if err != nil {
			return coreEntity.InternalServerError("Internal server error").SendResponse(c)
		}
		switch entityType {
		case "client":
			return c.Next()
		case "principal", "distributor", "partner":
			denied := coreEntity.Forbidden("Forbidden")
			denied.Data = fiber.Map{"reason": reasonEntityType}
			return denied.SendResponse(c)
		default:
			return coreEntity.InternalServerError("Internal server error").SendResponse(c)
		}
	}
}

// clientWritable refuses a write from a Suspended or Expired client (SD-5); reads are not gated.
func clientWritable(h *handler.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		status, err := h.ClientStatus(c.Context(), c.Get(constant.HEADER_CLIENT_ID))
		if err != nil {
			return coreEntity.InternalServerError("Internal server error").SendResponse(c)
		}
		switch status {
		case "Suspended", "Expired":
			denied := coreEntity.Forbidden("Forbidden")
			denied.Data = fiber.Map{"reason": reasonClientSuspended}
			return denied.SendResponse(c)
		default:
			return c.Next()
		}
	}
}

// notReadOnly rejects the Read Only access level: it limits every right to Read, so an Update
// role held next to it does not allow a write.
func notReadOnly(_ fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	return !roles.IsReadOnly(enum.ModuleAccessFixedAsset), nil
}

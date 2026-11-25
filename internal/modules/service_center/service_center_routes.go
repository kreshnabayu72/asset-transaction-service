package service_center

import (
	"github.com/gofiber/fiber/v3"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
)

const checkAvailabilityPath = "/service-centers/check-availability"

func SetupPublicRoutes(app *fiber.App, dbName string, dispatchers ...service.StatusChangeLogDispatcher) {
	repo := repository.NewServiceCenterRepository(dbName)
	checkAvailabilityService := service.NewCheckAvailabilityService(repo)
	checkAvailabilityHandler := handler.NewCheckAvailabilityHandler(checkAvailabilityService)

	app.Post(checkAvailabilityPath, requireServiceCenterPermission(canCreateOrEditServiceCenter), checkAvailabilityHandler.CheckAvailability)

	// Status has a local transactional implementation; unresolved gates and side effects remain documented DQs.
	var dispatcher service.StatusChangeLogDispatcher
	if len(dispatchers) > 0 {
		dispatcher = dispatchers[0]
	}
	SetupStatusRoutes(app, dbName, dispatcher)
}

func requireServiceCenterPermission(rule func(fiber.Ctx, middleware.UserAccessCache) (bool, error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		roles, ok := middleware.GetUserRoles(c)
		if !ok {
			return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
		}
		allowed, err := rule(c, roles)
		if err != nil {
			return coreEntity.ToHttpError(err).SendResponse(c)
		}
		if !allowed {
			denied := coreEntity.Forbidden("Forbidden")
			denied.Data = map[string]string{"reason": "capability"}
			return denied.SendResponse(c)
		}
		return c.Next()
	}
}

func canCreateOrEditServiceCenter(c fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	if roles.IsTotalControl(enum.ModuleAccessFixedAsset) {
		return true, nil
	}

	for _, verb := range []middleware.PermissionVerb{middleware.PermCreate, middleware.PermEdit} {
		if len(roles.GroupIDsWithPermission(verb, enum.SystemRoleKeyManageGroup)) > 0 {
			return true, nil
		}

		adminGroups := roles.GroupIDsWithPermission(verb, enum.SystemRoleKeyManageAssetAndTAG)
		for _, groupID := range adminGroups {
			if hasServiceCenterTransactionRole(roles, groupID) {
				return true, nil
			}
		}
	}
	return false, nil
}

func hasServiceCenterTransactionRole(roles middleware.UserAccessCache, groupID int64) bool {
	roleTypes := []enum.TransactionRolesType{
		enum.TransactionRolesTypeManager,
		enum.TransactionRolesTypeMonitoring,
		enum.TransactionRolesTypeStaff,
	}
	return roles.HasTransactionRoleFixedAsset(enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole, groupID, roleTypes...) ||
		roles.HasTransactionRoleFixedAsset(enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole, groupID, roleTypes...)
}

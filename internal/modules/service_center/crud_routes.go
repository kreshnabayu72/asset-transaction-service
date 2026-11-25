package service_center

import (
	"github.com/gofiber/fiber/v3"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
)

const (
	serviceCenterCollectionPath = "/service-centers"
	serviceCenterItemPath       = "/service-centers/:id"
)

// SetupCRUDRoutes wires the operation-specific public Create, Edit and Delete
// handlers. The caller supplies a Snowflake generator with its deployment node ID.
func SetupCRUDRoutes(app *fiber.App, dbName string, ids service.CRUDIDGenerator) {
	repo := repository.NewServiceCenterCRUDRepository(dbName)
	crudService := service.NewCRUDService(repo, ids)
	crudHandler := handler.NewCRUDHandler(crudService)

	app.Post(serviceCenterCollectionPath, requireServiceCenterPermission(canCreateServiceCenter), crudHandler.CreateServiceCenter)
	app.Put(serviceCenterItemPath, requireServiceCenterPermission(canEditServiceCenter), crudHandler.UpdateServiceCenter)
	app.Delete(serviceCenterItemPath, requireServiceCenterPermission(canDeleteServiceCenter), crudHandler.DeleteServiceCenter)
}

func canCreateServiceCenter(c fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	return hasServiceCenterCRUDRight(roles, middleware.PermCreate), nil
}

func canEditServiceCenter(c fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	return hasServiceCenterCRUDRight(roles, middleware.PermEdit), nil
}

func canDeleteServiceCenter(c fiber.Ctx, roles middleware.UserAccessCache) (bool, error) {
	return hasServiceCenterCRUDRight(roles, middleware.PermDelete), nil
}

func hasServiceCenterCRUDRight(roles middleware.UserAccessCache, verb middleware.PermissionVerb) bool {
	if roles.IsTotalControl(enum.ModuleAccessFixedAsset) {
		return true
	}
	if len(roles.GroupIDsWithPermission(verb, enum.SystemRoleKeyManageGroup)) > 0 {
		return true
	}
	for _, groupID := range roles.GroupIDsWithPermission(verb, enum.SystemRoleKeyManageAssetAndTAG) {
		if hasServiceCenterTransactionRole(roles, groupID) {
			return true
		}
	}
	return false
}

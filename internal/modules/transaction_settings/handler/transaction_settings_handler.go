// Package handler binds the transaction settings HTTP requests.
package handler

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"github.com/gofiber/fiber/v3"
)

// PlanReader returns the caller entity's plan for the request; an error (a provider failure) answers 500.
type PlanReader func(c fiber.Ctx) (service.Plan, error)

// Identity is the caller of a request: tenant, user id and full name.
type Identity struct {
	ClientID string
	UserID   int64
	FullName string
	// Access is the caller's rights on overrides, derived from the roles snapshot.
	Access service.CallerAccess
}

// IdentityReader returns the caller identity; an error answers 401.
type IdentityReader func(c fiber.Ctx) (Identity, error)

// Handler serves the transaction settings routes.
type Handler struct {
	svc      *service.Service
	plan     PlanReader
	identity IdentityReader
}

// NewHandler builds the handler; the caller identity comes from the LoadUserRoles snapshot.
func NewHandler(svc *service.Service, plan PlanReader) *Handler {
	return &Handler{svc: svc, plan: plan, identity: requestIdentity}
}

// PlanFromProvider builds the PlanReader of a plan provider: the caller entity is the request's client id.
func PlanFromProvider(read func(ctx context.Context, clientID string) (service.Plan, error)) PlanReader {
	return func(c fiber.Ctx) (service.Plan, error) {
		return read(c.Context(), c.Get(constant.HEADER_CLIENT_ID))
	}
}

// WithIdentity replaces the identity reader (tests).
func (h *Handler) WithIdentity(r IdentityReader) *Handler {
	h.identity = r
	return h
}

func requestIdentity(c fiber.Ctx) (Identity, error) {
	clientID, roles, userID, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return Identity{}, err
	}
	return Identity{ClientID: clientID, UserID: userID, FullName: roles.FullName, Access: callerAccess(roles)}, nil
}

// callerAccess derives the override rights of the caller from its roles snapshot.
func callerAccess(roles tsMiddleware.UserAccessCache) service.CallerAccess {
	totalControl := roles.IsTotalControl(enum.ModuleAccessFixedAsset)
	readOnly := roles.IsReadOnly(enum.ModuleAccessFixedAsset)
	manageUpdate := roles.HasAnySystemPermissionFixedAsset(tsMiddleware.PermEdit, enum.SystemRoleKeyManageTransactionSettings)
	manageView := roles.HasAnySystemPermissionFixedAsset(tsMiddleware.PermView, enum.SystemRoleKeyManageTransactionSettings)
	writeGroups := roles.GroupIDsWithPermission(tsMiddleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG)
	viewGroups := roles.GroupIDsWithPermission(tsMiddleware.PermView, enum.SystemRoleKeyManageAssetAndTAG)
	return service.CallerAccess{
		ReadOnly:       readOnly,
		TotalControl:   totalControl,
		ViewAll:        totalControl || readOnly || manageView,
		AssetNameWrite: totalControl || manageUpdate,
		WriteGroupIDs:  writeGroups,
		ViewGroupIDs:   append(slices.Clone(viewGroups), writeGroups...),
	}
}

const msgInvalidBody = "Invalid request body"

var errMalformedCategoryID = errors.New("categoryId must be a positive numeric id")

// parseCategoryID turns an optional decimal Snowflake id into *int64; empty selects All categories.
func parseCategoryID(raw *string) (*int64, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	for _, r := range *raw {
		if r < '0' || r > '9' {
			return nil, errMalformedCategoryID
		}
	}
	id, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil || id <= 0 {
		return nil, errMalformedCategoryID
	}
	return &id, nil
}

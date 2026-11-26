package service_center

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

func TestCanCreateOrEditServiceCenter(t *testing.T) {
	tests := []struct {
		name  string
		roles middleware.UserAccessCache
		allow bool
	}{
		{
			name: "group manager with create right",
			roles: middleware.UserAccessCache{FixedAssetSystemRole: struct {
				GlobalRole map[enum.SystemRoleKey]middleware.Permissions[bool]     `json:"globalRole"`
				GroupRoles map[enum.SystemRoleKey]middleware.Permissions[[]string] `json:"groupRoles"`
			}{GroupRoles: map[enum.SystemRoleKey]middleware.Permissions[[]string]{
				enum.SystemRoleKeyManageGroup: {Create: []string{"11"}},
			}}},
			allow: true,
		},
		{
			name: "asset admin and transaction role on the same group",
			roles: middleware.UserAccessCache{
				FixedAssetSystemRole: struct {
					GlobalRole map[enum.SystemRoleKey]middleware.Permissions[bool]     `json:"globalRole"`
					GroupRoles map[enum.SystemRoleKey]middleware.Permissions[[]string] `json:"groupRoles"`
				}{GroupRoles: map[enum.SystemRoleKey]middleware.Permissions[[]string]{
					enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"11"}},
				}},
				FixedAssetTransactionRole: map[enum.FixedAssetTransactionRoleTransactionKey]map[enum.TransactionRolesType][]string{
					enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole: {
						enum.TransactionRolesTypeStaff: {"11"},
					},
				},
			},
			allow: true,
		},
		{
			name: "asset admin and transaction role on different groups",
			roles: middleware.UserAccessCache{
				FixedAssetSystemRole: struct {
					GlobalRole map[enum.SystemRoleKey]middleware.Permissions[bool]     `json:"globalRole"`
					GroupRoles map[enum.SystemRoleKey]middleware.Permissions[[]string] `json:"groupRoles"`
				}{GroupRoles: map[enum.SystemRoleKey]middleware.Permissions[[]string]{
					enum.SystemRoleKeyManageAssetAndTAG: {Create: []string{"11"}},
				}},
				FixedAssetTransactionRole: map[enum.FixedAssetTransactionRoleTransactionKey]map[enum.TransactionRolesType][]string{
					enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole: {
						enum.TransactionRolesTypeManager: {"12"},
					},
				},
			},
			allow: false,
		},
		{name: "no create or edit right", allow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := canCreateOrEditServiceCenter(fiber.Ctx(nil), tt.roles)
			require.NoError(t, err)
			require.Equal(t, tt.allow, allowed)
		})
	}
}

package handler

import (
	"reflect"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

func TestParseStatusPathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
		ok   bool
	}{
		{name: "positive decimal", raw: "1924881519926425600", want: 1924881519926425600, ok: true},
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-1"},
		{name: "not decimal", raw: "x"},
		{name: "overflow", raw: "999999999999999999999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseStatusPathID(tt.raw)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parseStatusPathID(%q) = (%d, %v), want (%d, %v)", tt.raw, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestStatusAuthorizationFromSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		roles      tsMiddleware.UserAccessCache
		wantManage []int64
		wantAdmin  []int64
		total      bool
		right      bool
	}{
		{
			name:       "group manager edit right",
			roles:      statusTestRoles(enum.SystemRoleKeyManageGroup, "11", nil),
			wantManage: []int64{11},
			right:      true,
		},
		{
			name: "asset admin needs a transaction role on the same group",
			roles: statusTestRoles(
				enum.SystemRoleKeyManageAssetAndTAG,
				"11",
				map[enum.FixedAssetTransactionRoleTransactionKey]map[enum.TransactionRolesType][]string{
					enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole: {
						enum.TransactionRolesTypeStaff: []string{"11"},
					},
				},
			),
			wantAdmin: []int64{11},
			right:     true,
		},
		{
			name: "asset admin role on another group grants no status right",
			roles: statusTestRoles(
				enum.SystemRoleKeyManageAssetAndTAG,
				"11",
				map[enum.FixedAssetTransactionRoleTransactionKey]map[enum.TransactionRolesType][]string{
					enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole: {
						enum.TransactionRolesTypeManager: []string{"12"},
					},
				},
			),
			right: false,
		},
		{
			name: "total control",
			roles: tsMiddleware.UserAccessCache{
				ModuleAccess: map[enum.ModuleAccess]tsMiddleware.ModuleFlags{
					enum.ModuleAccessFixedAsset: {IsTotalControl: true},
				},
			},
			total: true,
			right: true,
		},
		{name: "no status capability"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusAuthorizationFromSnapshot(tt.roles)
			if !reflect.DeepEqual(got.ManageGroupIDs, tt.wantManage) {
				t.Fatalf("manage group IDs = %v, want %v", got.ManageGroupIDs, tt.wantManage)
			}
			if !reflect.DeepEqual(got.AssetAdminGroupIDs, tt.wantAdmin) {
				t.Fatalf("asset admin group IDs = %v, want %v", got.AssetAdminGroupIDs, tt.wantAdmin)
			}
			if got.TotalControl != tt.total {
				t.Fatalf("total control = %v, want %v", got.TotalControl, tt.total)
			}
			if got.HasStatusRight() != tt.right {
				t.Fatalf("HasStatusRight() = %v, want %v", got.HasStatusRight(), tt.right)
			}
		})
	}
}

func statusTestRoles(
	key enum.SystemRoleKey,
	groupID string,
	transactionRoles map[enum.FixedAssetTransactionRoleTransactionKey]map[enum.TransactionRolesType][]string,
) tsMiddleware.UserAccessCache {
	return tsMiddleware.UserAccessCache{
		FixedAssetSystemRole: struct {
			GlobalRole map[enum.SystemRoleKey]tsMiddleware.Permissions[bool]     `json:"globalRole"`
			GroupRoles map[enum.SystemRoleKey]tsMiddleware.Permissions[[]string] `json:"groupRoles"`
		}{
			GroupRoles: map[enum.SystemRoleKey]tsMiddleware.Permissions[[]string]{
				key: {Edit: []string{groupID}},
			},
		},
		FixedAssetTransactionRole: transactionRoles,
	}
}

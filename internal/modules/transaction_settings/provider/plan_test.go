package provider

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// planFixture is a fake of the Fixed Asset subscription, the addon ledger and iam-service approver presence.
type planFixture struct {
	subscriptions []emshttp.SubscriptionItem
	licenses      []emshttp.AddonLicense
	presence      map[string]bool
	failSubs      bool
	failCaps      bool
	failPresence  bool
	presenceCalls atomic.Int32
}

func (f *planFixture) provider(t *testing.T) PlanProvider {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+pathSubscriptions, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if f.failSubs {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, emshttp.SubscriptionsData{Subscriptions: f.subscriptions})
	})
	mux.HandleFunc("GET "+pathEffectiveCapacities, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if f.failCaps {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, emshttp.EffectiveCapacitiesData{AddonLicenses: f.licenses})
	})
	mux.HandleFunc("POST "+pathApproverPresence, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		f.presenceCalls.Add(1)
		if f.failPresence {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, f.presence)
	})
	base := serve(t, mux)
	return PlanProvider{ems: emsClient{client: testClient(base)}, iam: iamClient{client: testClient(base)}}
}

// fixedAssetSub is a Fixed Asset subscription with the given status and fixed_asset_module rows.
func fixedAssetSub(status string, rows ...emshttp.SubscriptionFeatureRow) emshttp.SubscriptionItem {
	return emshttp.SubscriptionItem{ProductSlug: productFixedAsset, Status: status, Features: rows}
}

func featureRow(feature, value string) emshttp.SubscriptionFeatureRow {
	return emshttp.SubscriptionFeatureRow{Section: sectionFixedAssetModule, Feature: feature, Value: value, Source: "plan_default"}
}

func TestPlanWithoutAnActiveFixedAssetSubscriptionHasNoEntitlements(t *testing.T) {
	cases := map[string][]emshttp.SubscriptionItem{
		"no subscriptions": nil,
		"expired":          {fixedAssetSub("expired", featureRow(featureBorrowing, constants.LevelFull))},
		"other product":    {{ProductSlug: "supply_asset", Status: "active"}},
	}
	for name, subs := range cases {
		t.Run(name, func(t *testing.T) {
			f := &planFixture{subscriptions: subs, licenses: []emshttp.AddonLicense{{LicenseType: "audit", Total: 1}}}
			plan, err := f.provider(t).Plan(context.Background(), testClientID)
			require.NoError(t, err)
			require.Equal(t, noPlan(), plan)
			require.Zero(t, f.presenceCalls.Load())
		})
	}
}

func TestPlanReadsFeatureSuitesApprovalAddonsAndPresence(t *testing.T) {
	f := &planFixture{
		subscriptions: []emshttp.SubscriptionItem{fixedAssetSub("active",
			featureRow(featureApprovalWorkflow, "true"),
			featureRow(featureBorrowing, constants.LevelFull),
			featureRow(featureAssignment, constants.LevelBasic),
			featureRow(featureDisposal, constants.LevelBasic),
		)},
		licenses: []emshttp.AddonLicense{{LicenseType: addonAudit, Total: 2}, {LicenseType: addonMaintenance, Total: 0}},
		presence: map[string]bool{constants.TransactionBorrow: true},
	}
	plan, err := f.provider(t).Plan(context.Background(), testClientID)
	require.NoError(t, err)
	require.Equal(t, service.Plan{
		ApprovalWorkflowAvailable: true,
		FeatureSuite: dto.FeatureSuite{
			Borrowing:  constants.LevelFull,
			Assignment: constants.LevelBasic,
			Transfer:   constants.LevelNone,
			Disposal:   constants.LevelBasic,
		},
		Addons:      dto.Addons{Audit: true},
		HasApprover: map[string]bool{constants.TransactionBorrow: true},
	}, plan)
	require.EqualValues(t, 1, f.presenceCalls.Load())
}

func TestPlanGraceSubscriptionCountsAndApprovalOffSkipsPresence(t *testing.T) {
	f := &planFixture{
		subscriptions: []emshttp.SubscriptionItem{fixedAssetSub("grace", featureRow(featureApprovalWorkflow, "false"))},
		licenses:      []emshttp.AddonLicense{{LicenseType: addonMaintenance, Total: 3}},
	}
	plan, err := f.provider(t).Plan(context.Background(), testClientID)
	require.NoError(t, err)
	require.False(t, plan.ApprovalWorkflowAvailable)
	require.True(t, plan.Addons.Maintenance)
	require.Nil(t, plan.HasApprover)
	require.Zero(t, f.presenceCalls.Load())
}

func TestPlanSurfacesProviderFailuresAndContractValues(t *testing.T) {
	active := []emshttp.SubscriptionItem{fixedAssetSub("active", featureRow(featureApprovalWorkflow, "true"))}
	cases := map[string]*planFixture{
		"subscriptions down":  {failSubs: true},
		"capacities down":     {subscriptions: active, failCaps: true},
		"presence down":       {subscriptions: active, failPresence: true},
		"unknown approval":    {subscriptions: []emshttp.SubscriptionItem{fixedAssetSub("active", featureRow(featureApprovalWorkflow, "maybe"))}},
		"unknown suite level": {subscriptions: []emshttp.SubscriptionItem{fixedAssetSub("active", featureRow(featureBorrowing, "gold"))}},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.provider(t).Plan(context.Background(), testClientID)
			require.Error(t, err)
		})
	}
}

func TestAddonsRequirePositiveTotals(t *testing.T) {
	require.Equal(t, dto.Addons{Audit: true, Maintenance: true}, addonsOf([]emshttp.AddonLicense{
		{LicenseType: addonAudit, Total: 1},
		{LicenseType: addonMaintenance, Total: 5},
		{LicenseType: "tracking", Total: 9},
	}))
	require.Equal(t, dto.Addons{}, addonsOf([]emshttp.AddonLicense{{LicenseType: addonAudit, Total: 0}, {LicenseType: addonMaintenance, Total: -1}}))
}

package provider

import (
	"context"
	"fmt"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// Values of the Fixed Asset subscription the plan reads (enterprise-management GET /internal/subscriptions).
const (
	subscriptionActive      = "active"
	subscriptionGrace       = "grace"
	sectionFixedAssetModule = "fixed_asset_module"
	featureApprovalWorkflow = "approval_workflow"
	featureBorrowing        = "borrowing"
	featureAssignment       = "assignment"
	featureTransfer         = "transfer"
	featureDisposal         = "disposal"
	addonAudit              = "audit"
	addonMaintenance        = "maintenance"
)

// approverTransactions are the transactions whose Transaction Role Approver presence the page reads (LLD GET step 9).
var approverTransactions = []string{
	constants.TransactionBorrow,
	constants.TransactionAssign,
	constants.TransactionTransfer,
	constants.TransactionDisposal,
	constants.TransactionAudit,
	constants.TransactionMaintenance,
}

// PlanProvider reads the entity plan of the settings page: Approval Workflow, the Feature Suite levels and the
// Audit and Maintenance addons from the Fixed Asset subscription, and the Approver presence from iam-service.
type PlanProvider struct {
	ems emsClient
	iam iamClient
}

// Plan answers the plan of the caller's entity. An entity without an active or grace Fixed Asset subscription
// has no plan: every Feature Suite is None, Approval Workflow and both addons are off, and no presence is read.
func (p PlanProvider) Plan(ctx context.Context, clientID string) (service.Plan, error) {
	subscriptions, err := p.ems.subscriptions(ctx, clientID)
	if err != nil {
		return service.Plan{}, fmt.Errorf("read subscriptions: %w", err)
	}
	rows, ok := activeFixedAssetRows(subscriptions)
	if !ok {
		return noPlan(), nil
	}
	plan, err := planFromRows(rows)
	if err != nil {
		return service.Plan{}, err
	}
	capacities, err := p.ems.effectiveCapacities(ctx, clientID)
	if err != nil {
		return service.Plan{}, fmt.Errorf("read effective capacities: %w", err)
	}
	plan.Addons = addonsOf(capacities.AddonLicenses)
	if !plan.ApprovalWorkflowAvailable {
		return plan, nil
	}
	presence, err := p.iam.approverPresence(ctx, clientID, approverTransactions)
	if err != nil {
		return service.Plan{}, fmt.Errorf("read approver presence: %w", err)
	}
	plan.HasApprover = presence
	return plan, nil
}

// noPlan is the plan of an entity without an active Fixed Asset subscription.
func noPlan() service.Plan {
	return service.Plan{FeatureSuite: dto.FeatureSuite{
		Borrowing:  constants.LevelNone,
		Assignment: constants.LevelNone,
		Transfer:   constants.LevelNone,
		Disposal:   constants.LevelNone,
	}}
}

// activeFixedAssetRows returns the fixed_asset_module rows of the first active or grace Fixed Asset subscription.
func activeFixedAssetRows(subscriptions []emshttp.SubscriptionItem) (map[string]string, bool) {
	for _, sub := range subscriptions {
		if sub.ProductSlug != productFixedAsset || (sub.Status != subscriptionActive && sub.Status != subscriptionGrace) {
			continue
		}
		rows := make(map[string]string, len(sub.Features))
		for _, row := range sub.Features {
			if row.Section == sectionFixedAssetModule {
				rows[row.Feature] = row.Value
			}
		}
		return rows, true
	}
	return nil, false
}

// planFromRows reads Approval Workflow and the Feature Suite levels of one subscription. A feature with no row is
// not entitled (Approval Workflow off, level None); a value outside its vocabulary fails the read.
func planFromRows(rows map[string]string) (service.Plan, error) {
	approval, err := boolRow(rows, featureApprovalWorkflow)
	if err != nil {
		return service.Plan{}, err
	}
	var suite dto.FeatureSuite
	levels := []struct {
		feature string
		target  *string
	}{
		{featureBorrowing, &suite.Borrowing},
		{featureAssignment, &suite.Assignment},
		{featureTransfer, &suite.Transfer},
		{featureDisposal, &suite.Disposal},
	}
	for _, level := range levels {
		value, err := levelRow(rows, level.feature)
		if err != nil {
			return service.Plan{}, err
		}
		*level.target = value
	}
	return service.Plan{ApprovalWorkflowAvailable: approval, FeatureSuite: suite}, nil
}

// boolRow reads a boolean feature row ("true" or "false"); a missing row is false.
func boolRow(rows map[string]string, feature string) (bool, error) {
	raw, ok := rows[feature]
	if !ok {
		return false, nil
	}
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("%w: %s is %q", errContract, feature, raw)
}

// levelRow reads a Feature Suite row (none, basic or full); a missing row is None.
func levelRow(rows map[string]string, feature string) (string, error) {
	raw, ok := rows[feature]
	if !ok {
		return constants.LevelNone, nil
	}
	switch raw {
	case constants.LevelNone, constants.LevelBasic, constants.LevelFull:
		return raw, nil
	}
	return "", fmt.Errorf("%w: %s is %q", errContract, feature, raw)
}

// addonsOf reports the Audit and Maintenance addons as active when their licence total is positive.
func addonsOf(licenses []emshttp.AddonLicense) dto.Addons {
	var addons dto.Addons
	for _, license := range licenses {
		if license.Total <= 0 {
			continue
		}
		switch license.LicenseType {
		case addonAudit:
			addons.Audit = true
		case addonMaintenance:
			addons.Maintenance = true
		}
	}
	return addons
}

package service

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// ReadTransactionSettings returns the rules of one node (nil categoryID = All categories)
// for every transaction, in tab order, with the plan's availability and gating. Stored rows are
// merged over the catalogue defaults: All categories returns the Company default own values; a
// leaf returns its own values and every other field as Inherit with the Company default value.
func (s *Service) ReadTransactionSettings(ctx context.Context, clientID string, categoryID *int64, plan Plan) (dto.ReadSettingsData, error) {
	if _, err := s.resolveLeafCategory(ctx, clientID, categoryID); err != nil {
		return dto.ReadSettingsData{}, err
	}
	version, err := s.repo.FindSettingsVersion(ctx, clientID)
	if err != nil {
		return dto.ReadSettingsData{}, fmt.Errorf("read settings version: %w", err)
	}
	companyRows, err := s.repo.FindNodeFields(ctx, clientID, nil)
	if err != nil {
		return dto.ReadSettingsData{}, fmt.Errorf("read company default fields: %w", err)
	}
	company, leaf := newFieldIndex(companyRows), fieldIndex{}
	if categoryID != nil {
		leafRows, err := s.repo.FindNodeFields(ctx, clientID, categoryID)
		if err != nil {
			return dto.ReadSettingsData{}, fmt.Errorf("read category fields: %w", err)
		}
		leaf = newFieldIndex(leafRows)
	}
	data := dto.ReadSettingsData{
		SettingsVersion:           int(version),
		ApprovalWorkflowAvailable: plan.ApprovalWorkflowAvailable,
		FeatureSuite:              plan.FeatureSuite,
		Addons:                    plan.Addons,
		Transactions:              make([]dto.TransactionSettings, 0, len(constants.TransactionOrder)),
	}
	if categoryID != nil {
		id := strconv.FormatInt(*categoryID, 10)
		data.CategoryID = &id
	}
	for _, tx := range constants.TransactionOrder {
		if categoryID != nil && slices.Contains(constants.AllCategoriesOnlyTransactions, tx) {
			continue
		}
		data.Transactions = append(data.Transactions, nodeTransaction(tx, categoryID != nil, plan, company, leaf))
	}
	return data, nil
}

// nodeTransaction builds one transaction block; an unavailable transaction has no fields.
func nodeTransaction(tx string, atLeaf bool, plan Plan, company, leaf fieldIndex) dto.TransactionSettings {
	out := dto.TransactionSettings{Transaction: tx, Available: transactionLevel(tx, plan) != constants.LevelNone, Fields: []dto.SettingField{}}
	if !out.Available {
		return out
	}
	for _, f := range constants.Catalogue[tx] {
		if atLeaf && f.AllCategoriesOnly {
			continue
		}
		companyVal := companyValue(company, tx, f)
		field := dto.SettingField{Key: f.Key, Mode: constants.ModeOwn, Value: companyVal, DefaultValue: f.Default, Gated: isGated(tx, f.Key, plan)}
		if atLeaf {
			if own, ok := leaf.own(tx, f.Key); ok {
				field.Value = own
			} else {
				source := constants.CompanyDefaultSource
				field.Mode, field.Value, field.InheritedValue, field.Source = constants.ModeInherit, nil, companyVal, &source
			}
		}
		out.Fields = append(out.Fields, field)
	}
	out.NoApprover = noApprover(tx, out.Fields, plan)
	return out
}

// approvalKey is the field whose effective value says the transaction requires approval.
func approvalKey(tx string) string {
	if tx == constants.TransactionMaintenance {
		return constants.FieldRequireApprovalToClose
	}
	return constants.FieldRequireApproval
}

// noApprover is true when the transaction requires approval at the read level, Approval Workflow is
// available and the approver-presence provider reports no Transaction Role Approver (GET step 9).
func noApprover(tx string, fields []dto.SettingField, plan Plan) bool {
	if tx == constants.TransactionDamagedMissing || !plan.ApprovalWorkflowAvailable {
		return false
	}
	has, known := plan.HasApprover[tx]
	if !known || has {
		return false
	}
	key := approvalKey(tx)
	for _, f := range fields {
		if f.Key != key {
			continue
		}
		v := f.Value
		if f.Mode == constants.ModeInherit {
			v = f.InheritedValue
		}
		on, _ := v.(bool)
		return on
	}
	return false
}

// transactionLevel returns the Feature Suite level governing tx; Damaged & Missing is not
// plan-tiered, and Audit or Maintenance read as None while their addon is inactive.
func transactionLevel(tx string, plan Plan) string {
	switch tx {
	case constants.TransactionBorrow:
		return plan.FeatureSuite.Borrowing
	case constants.TransactionAssign:
		return plan.FeatureSuite.Assignment
	case constants.TransactionTransfer:
		return plan.FeatureSuite.Transfer
	case constants.TransactionDisposal:
		return plan.FeatureSuite.Disposal
	case constants.TransactionAudit:
		return addonLevel(plan.Addons.Audit)
	case constants.TransactionMaintenance:
		return addonLevel(plan.Addons.Maintenance)
	default:
		return constants.LevelFull
	}
}

func addonLevel(active bool) string {
	if active {
		return constants.LevelFull
	}
	return constants.LevelNone
}

// isGated reports whether the plan makes field key of tx unavailable (the stored value is kept).
func isGated(tx, key string, plan Plan) bool {
	if !plan.ApprovalWorkflowAvailable && slices.Contains(constants.ApprovalWorkflowGatedFields, key) {
		return true
	}
	return transactionLevel(tx, plan) == constants.LevelBasic && slices.Contains(constants.BasicGatedFields[tx], key)
}

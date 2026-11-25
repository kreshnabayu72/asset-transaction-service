package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

func fieldOf(t *testing.T, tx dto.TransactionSettings, key string) dto.SettingField {
	t.Helper()
	for _, f := range tx.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("field %s missing in %s", key, tx.Transaction)
	return dto.SettingField{}
}

func hasField(tx dto.TransactionSettings, key string) bool {
	for _, f := range tx.Fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

func TestReadTransactionSettings_AllCategoriesReturnsCompanyDefaults(t *testing.T) {
	data, err := NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", nil, FullPlan)
	require.NoError(t, err)

	require.Equal(t, 1, data.SettingsVersion)
	require.Nil(t, data.CategoryID)
	require.True(t, data.ApprovalWorkflowAvailable)
	require.Len(t, data.Transactions, 7)
	require.Equal(t, dto.Addons{Audit: true, Maintenance: true}, data.Addons)
	for i, tx := range constants.TransactionOrder {
		require.Equal(t, tx, data.Transactions[i].Transaction)
		require.True(t, data.Transactions[i].Available)
	}
	borrow := data.Transactions[0]
	maxDuration := fieldOf(t, borrow, constants.FieldMaxDuration)
	require.Equal(t, constants.ModeOwn, maxDuration.Mode)
	require.Equal(t, 14, maxDuration.Value)
	require.Equal(t, 14, maxDuration.DefaultValue)
	require.Nil(t, maxDuration.InheritedValue)
	require.Nil(t, maxDuration.Source)
	require.False(t, maxDuration.Gated)
	require.Equal(t, 7, fieldOf(t, borrow, constants.FieldTransactionTimeout).Value)
	require.Equal(t, true, fieldOf(t, borrow, constants.FieldSendEmailNotifications).Value)
	require.Equal(t, false, fieldOf(t, data.Transactions[1], constants.FieldRequireApproval).Value)
	require.Equal(t, constants.ValueBoth, fieldOf(t, data.Transactions[2], constants.FieldApprovalScope).Value)
	require.Equal(t, 30, fieldOf(t, data.Transactions[4], constants.FieldLongMissingAfter).Value)
}

func TestReadTransactionSettings_LeafInheritsWithoutAllCategoriesOnlyFields(t *testing.T) {
	id := int64(1924881519926425600)
	data, err := NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
	require.NoError(t, err)

	require.Equal(t, "1924881519926425600", *data.CategoryID)
	require.Len(t, data.Transactions, 5)
	require.Equal(t, constants.TransactionMaintenance, data.Transactions[4].Transaction)
	require.False(t, hasField(data.Transactions[4], constants.FieldRequireLocation))
	require.True(t, hasField(data.Transactions[4], constants.FieldPhotoAfter))
	borrow := data.Transactions[0]
	maxDuration := fieldOf(t, borrow, constants.FieldMaxDuration)
	require.Equal(t, constants.ModeInherit, maxDuration.Mode)
	require.Nil(t, maxDuration.Value)
	require.Equal(t, 14, maxDuration.InheritedValue)
	require.Equal(t, constants.CompanyDefaultSource, *maxDuration.Source)
	require.False(t, hasField(borrow, constants.FieldTransactionTimeout))
	require.False(t, hasField(borrow, constants.FieldSendEmailNotifications))
	require.False(t, hasField(data.Transactions[2], constants.FieldChangeCostCenterWithGroup))
}

func TestReadTransactionSettings_PlanNoneAndGating(t *testing.T) {
	plan := Plan{
		ApprovalWorkflowAvailable: false,
		FeatureSuite: dto.FeatureSuite{
			Borrowing:  constants.LevelBasic,
			Assignment: constants.LevelBasic,
			Transfer:   constants.LevelNone,
			Disposal:   constants.LevelFull,
		},
	}
	data, err := NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", nil, plan)
	require.NoError(t, err)

	borrow, assign, transfer, disposal := data.Transactions[0], data.Transactions[1], data.Transactions[2], data.Transactions[3]
	require.False(t, transfer.Available)
	require.Empty(t, transfer.Fields)
	require.True(t, fieldOf(t, borrow, constants.FieldSelfService).Gated)
	require.True(t, fieldOf(t, borrow, constants.FieldDurationUnit).Gated)
	require.True(t, fieldOf(t, borrow, constants.FieldRequireApproval).Gated)
	require.Equal(t, true, fieldOf(t, borrow, constants.FieldRequireApproval).Value)
	require.False(t, fieldOf(t, borrow, constants.FieldMaxDuration).Gated)
	require.True(t, fieldOf(t, assign, constants.FieldAllowedAssigneeEmployee).Gated)
	require.False(t, fieldOf(t, assign, constants.FieldPhotoAtStart).Gated)
	require.True(t, fieldOf(t, disposal, constants.FieldApprovalType).Gated)
	require.False(t, fieldOf(t, disposal, constants.FieldAllowDisposal).Gated)
	require.True(t, data.Transactions[4].Available)
	require.False(t, data.Transactions[5].Available)
	require.Empty(t, data.Transactions[5].Fields)
	require.False(t, data.Transactions[6].Available)
}

func TestReadTransactionSettings_AuditAndMaintenanceDefaults(t *testing.T) {
	data, err := NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", nil, FullPlan)
	require.NoError(t, err)
	audit, maint := data.Transactions[5], data.Transactions[6]
	require.Equal(t, constants.TransactionAudit, audit.Transaction)
	require.Equal(t, 3, fieldOf(t, audit, constants.FieldGracePeriod).Value)
	require.Equal(t, true, fieldOf(t, audit, constants.FieldRequireApproval).Value)
	require.Equal(t, false, fieldOf(t, audit, constants.FieldRequireLocation).Value)
	require.False(t, hasField(audit, constants.FieldTransactionTimeout))
	require.Equal(t, constants.ValueAnyOneApprover, fieldOf(t, maint, constants.FieldApprovalType).Value)
	require.Equal(t, 3, fieldOf(t, maint, constants.FieldReminderBeforeDue).Value)
	require.Equal(t, true, fieldOf(t, data.Transactions[1], constants.FieldNeedRecipientConfirmation).Value)
	require.Equal(t, true, fieldOf(t, data.Transactions[0], constants.FieldAllowBorrow).Value)
	require.Equal(t, constants.ValueDays, fieldOf(t, data.Transactions[0], constants.FieldDurationUnit).Value)
	require.Equal(t, []string{"sell", "scrap", "donate", "trade_in", "write_off"}, fieldOf(t, data.Transactions[3], constants.FieldAllowedDisposalTypes).Value)
}

func TestReadTransactionSettings_NoApprover(t *testing.T) {
	plan := FullPlan
	plan.HasApprover = map[string]bool{"borrow": false, "assign": false, "transfer": true, "maintenance": false}
	data, err := NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", nil, plan)
	require.NoError(t, err)
	require.True(t, data.Transactions[0].NoApprover)
	require.False(t, data.Transactions[1].NoApprover) // require_approval OFF
	require.False(t, data.Transactions[2].NoApprover) // has approver
	require.False(t, data.Transactions[3].NoApprover) // presence unknown
	require.False(t, data.Transactions[4].NoApprover) // damaged_missing
	require.False(t, data.Transactions[6].NoApprover) // require_approval_to_close OFF

	plan.ApprovalWorkflowAvailable = false
	data, err = NewService(&fakeRepo{}, nil).ReadTransactionSettings(context.Background(), "c1", nil, plan)
	require.NoError(t, err)
	require.False(t, data.Transactions[0].NoApprover)
}

func ownRow(tx, key, value string) model.TransactionRuleField {
	return model.TransactionRuleField{TransactionType: tx, FieldKey: key, Mode: constants.ModeOwn, Value: raw(value)}
}

func TestReadTransactionSettings_AllCategoriesMergesStoredRows(t *testing.T) {
	repo := &fakeRepo{version: 7, company: []model.TransactionRuleField{
		ownRow("borrow", constants.FieldMaxDuration, "30"),
		ownRow("disposal", constants.FieldAllowedDisposalTypes, `["sell","scrap"]`),
		{TransactionType: "borrow", FieldKey: constants.FieldMaxExtensions, Mode: constants.ModeInherit},
	}}
	data, err := NewService(repo, nil).ReadTransactionSettings(context.Background(), "c1", nil, FullPlan)
	require.NoError(t, err)

	require.Equal(t, 7, data.SettingsVersion)
	maxDuration := fieldOf(t, data.Transactions[0], constants.FieldMaxDuration)
	require.Equal(t, 30, maxDuration.Value)
	require.Equal(t, 14, maxDuration.DefaultValue)
	require.Equal(t, constants.ModeOwn, maxDuration.Mode)
	require.Equal(t, []string{"sell", "scrap"}, fieldOf(t, data.Transactions[3], constants.FieldAllowedDisposalTypes).Value)
	require.Equal(t, 1, fieldOf(t, data.Transactions[0], constants.FieldMaxExtensions).Value)
}

func TestReadTransactionSettings_LeafMergesOwnAndInherit(t *testing.T) {
	repo := &fakeRepo{
		company: []model.TransactionRuleField{ownRow("borrow", constants.FieldMaxDuration, "30")},
		leaf: []model.TransactionRuleField{
			ownRow("borrow", constants.FieldMaxExtensions, "0"),
			{TransactionType: "borrow", FieldKey: constants.FieldMaxAssetsPerBorrower, Mode: constants.ModeInherit},
		},
	}
	id := int64(5)
	data, err := NewService(repo, nil).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
	require.NoError(t, err)

	borrow := data.Transactions[0]
	own := fieldOf(t, borrow, constants.FieldMaxExtensions)
	require.Equal(t, constants.ModeOwn, own.Mode)
	require.Equal(t, 0, own.Value)
	require.Nil(t, own.InheritedValue)
	require.Nil(t, own.Source)
	inherited := fieldOf(t, borrow, constants.FieldMaxDuration)
	require.Equal(t, constants.ModeInherit, inherited.Mode)
	require.Nil(t, inherited.Value)
	require.Equal(t, 30, inherited.InheritedValue)
	require.Equal(t, constants.CompanyDefaultSource, *inherited.Source)
	require.Equal(t, 3, fieldOf(t, borrow, constants.FieldMaxAssetsPerBorrower).InheritedValue)
}

func TestReadTransactionSettings_RepositoryErrors(t *testing.T) {
	id := int64(5)
	_, err := NewService(&fakeRepo{err: errors.New("db down")}, nil).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
	require.ErrorContains(t, err, "db down")
}

func TestReadTransactionSettings_CategoryLookup(t *testing.T) {
	id := int64(42)
	t.Run("an unknown, deleted, other-entity or parent Category is not found", func(t *testing.T) {
		for name, info := range map[string]CategoryInfo{"unknown, deleted or other entity": {}, "parent": {Exists: true}} {
			dir := &fakeCategories{infos: map[int64]CategoryInfo{42: info}}
			_, err := NewService(&fakeRepo{}, nil).WithCategoryDirectory(dir).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
			require.ErrorIs(t, err, ErrCategoryNotFound, name)
		}
	})
	t.Run("a leaf is read under its id", func(t *testing.T) {
		dir := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true, Path: "Vehicle > Pool car"}}}
		data, err := NewService(&fakeRepo{}, nil).WithCategoryDirectory(dir).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
		require.NoError(t, err)
		require.Equal(t, "42", *data.CategoryID)
	})
	t.Run("a directory failure is an error", func(t *testing.T) {
		dir := &fakeCategories{err: errBoom}
		_, err := NewService(&fakeRepo{}, nil).WithCategoryDirectory(dir).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
		require.ErrorIs(t, err, errBoom)
	})
}

func TestReadTransactionSettings_NodeReadFailures(t *testing.T) {
	id := int64(42)
	_, err := NewService(&failingRepo{failFields: 1}, nil).ReadTransactionSettings(context.Background(), "c1", nil, FullPlan)
	require.ErrorContains(t, err, "read company default fields")
	_, err = NewService(&failingRepo{failFields: 2}, nil).ReadTransactionSettings(context.Background(), "c1", &id, FullPlan)
	require.ErrorContains(t, err, "read category fields")
}

func TestNoApprover_Branches(t *testing.T) {
	plan := Plan{ApprovalWorkflowAvailable: true, HasApprover: map[string]bool{constants.TransactionBorrow: false}}
	key := approvalKey(constants.TransactionBorrow)
	// An inherited value counts: the Company default is ON, so nobody approves.
	require.True(t, noApprover(constants.TransactionBorrow, []dto.SettingField{{Key: key, Mode: constants.ModeInherit, InheritedValue: true}}, plan))
	// Without a row for the approval key there is no approval to flag.
	require.False(t, noApprover(constants.TransactionBorrow, []dto.SettingField{{Key: "max_duration"}}, plan))
}

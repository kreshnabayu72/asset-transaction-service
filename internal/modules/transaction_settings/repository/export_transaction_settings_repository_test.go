package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

func TestFindExportFields_AllCategoriesIsOneQuery(t *testing.T) {
	fake := &fakeServicer{fields: [][]model.TransactionRuleField{{{FieldKey: "a"}, {FieldKey: "b"}}}}
	rows, err := NewTransactionSettingsRepository(fake.factory()).FindExportFields(context.Background(), "c1", nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Len(t, fake.queries, 1)
	require.True(t, fake.closed)
}

func TestFindExportFields_LeafPrependsCompanyDefault(t *testing.T) {
	fake := &fakeServicer{fields: [][]model.TransactionRuleField{{{FieldKey: "company"}}, {{FieldKey: "leaf"}}}}
	id := int64(9)
	rows, err := NewTransactionSettingsRepository(fake.factory()).FindExportFields(context.Background(), "c1", &id)
	require.NoError(t, err)
	require.Equal(t, "company", rows[0].FieldKey)
	require.Equal(t, "leaf", rows[1].FieldKey)
}

func TestFindExportFields_Error(t *testing.T) {
	fake := &fakeServicer{err: errors.New("db down")}
	_, err := NewTransactionSettingsRepository(fake.factory()).FindExportFields(context.Background(), "c1", nil)
	require.ErrorContains(t, err, "db down")
}

func TestFindExportOverrides(t *testing.T) {
	fake := &fakeServicer{overrides: [][]model.TransactionRuleOverride{{{TargetName: "all"}}}}
	rows, err := NewTransactionSettingsRepository(fake.factory()).FindExportOverrides(context.Background(), "c1", nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, fake.queries, 1)

	fake = &fakeServicer{overrides: [][]model.TransactionRuleOverride{{{TargetName: "leaf"}}, {{TargetName: "company"}}}}
	id := int64(9)
	rows, err = NewTransactionSettingsRepository(fake.factory()).FindExportOverrides(context.Background(), "c1", &id)
	require.NoError(t, err)
	require.Equal(t, "leaf", rows[0].TargetName)
	require.Equal(t, "company", rows[1].TargetName)
	require.Len(t, fake.queries, 2)
}

func TestFindExportOverrides_Error(t *testing.T) {
	fake := &fakeServicer{err: errors.New("db down")}
	_, err := NewTransactionSettingsRepository(fake.factory()).FindExportOverrides(context.Background(), "c1", nil)
	require.ErrorContains(t, err, "db down")
}

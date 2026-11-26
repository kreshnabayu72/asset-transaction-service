package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

func TestFindNodeFields(t *testing.T) {
	fake := &fakeServicer{fields: [][]model.TransactionRuleField{{{FieldKey: "max_duration", Mode: "own"}}}}
	rows, err := NewTransactionSettingsRepository(fake.factory()).FindNodeFields(context.Background(), "c1", nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, fake.queries, 1)
	require.False(t, fake.closed)
}

func TestFindNodeFields_Error(t *testing.T) {
	fake := &fakeServicer{err: errors.New("db down")}
	id := int64(3)
	_, err := NewTransactionSettingsRepository(fake.factory()).FindNodeFields(context.Background(), "c1", &id)
	require.ErrorContains(t, err, "db down")
	require.False(t, fake.closed)
}

func TestFindSettingsVersion(t *testing.T) {
	repo := func(v any) (*TransactionSettingsRepository, *fakeServicer) {
		f := &fakeServicer{version: v}
		return NewTransactionSettingsRepository(f.factory()), f
	}
	r, _ := repo(int64(5))
	v, err := r.FindSettingsVersion(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, int64(5), v)

	r, _ = repo(pgx.ErrNoRows)
	v, err = r.FindSettingsVersion(context.Background(), "c1")
	require.NoError(t, err)
	require.Equal(t, InitialSettingsVersion, v)

	r, _ = repo(errors.New("boom"))
	_, err = r.FindSettingsVersion(context.Background(), "c1")
	require.ErrorContains(t, err, "boom")
}

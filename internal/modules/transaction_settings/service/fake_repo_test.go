package service

import (
	"context"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

// fakeRepo is an in-memory SettingsRepository; zero value = nothing stored, version 1.
type fakeRepo struct {
	version   int64
	company   []model.TransactionRuleField
	leaf      []model.TransactionRuleField
	export    []model.TransactionRuleField
	overrides []model.TransactionRuleOverride
	err       error
	tx        *fakeTx
	beginErr  error
	leafErr   error
	began     int
}

// fakeTx records the writes of one save.
type fakeTx struct {
	rows        []model.TransactionRuleField
	bumpedBy    string
	bumps       int
	committed   bool
	rolledBack  bool
	closed      bool
	upsertErr   error
	bumpErr     error
	commitErr   error
	rollbackErr error
}

func (t *fakeTx) UpsertFields(_ context.Context, rows []model.TransactionRuleField) error {
	t.rows = append(t.rows, rows...)
	return t.upsertErr
}

func (t *fakeTx) BumpSettingsVersion(_ context.Context, _ int64, name string) error {
	t.bumps++
	t.bumpedBy = name
	return t.bumpErr
}

func (t *fakeTx) Commit(context.Context) error   { t.committed = true; return t.commitErr }
func (t *fakeTx) Rollback(context.Context) error { t.rolledBack = true; return t.rollbackErr }
func (t *fakeTx) Close()                         { t.closed = true }

func (f *fakeRepo) BeginSave(context.Context, string) (SaveTx, error) {
	f.began++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.tx == nil {
		f.tx = &fakeTx{}
	}
	return f.tx, nil
}

func (f *fakeRepo) FindNodeFields(_ context.Context, _ string, categoryID *int64) ([]model.TransactionRuleField, error) {
	if categoryID == nil {
		return f.company, f.err
	}
	if f.leafErr != nil {
		return nil, f.leafErr
	}
	return f.leaf, f.err
}

func (f *fakeRepo) FindSettingsVersion(context.Context, string) (int64, error) {
	if f.version == 0 {
		return 1, f.err
	}
	return f.version, f.err
}

func (f *fakeRepo) FindExportFields(context.Context, string, *int64) ([]model.TransactionRuleField, error) {
	return f.export, f.err
}

func (f *fakeRepo) FindExportOverrides(context.Context, string, *int64) ([]model.TransactionRuleOverride, error) {
	return f.overrides, f.err
}

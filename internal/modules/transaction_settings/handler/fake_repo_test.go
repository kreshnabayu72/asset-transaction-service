package handler

import (
	"context"
	"errors"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// fakeRepo stores nothing; with fail set every read errors.
type fakeRepo struct {
	fail bool
	tx   *fakeTx
}

// fakeTx accepts every write.
type fakeTx struct{ rows int }

func (t *fakeTx) UpsertFields(_ context.Context, rows []model.TransactionRuleField) error {
	t.rows += len(rows)
	return nil
}
func (t *fakeTx) BumpSettingsVersion(context.Context, int64, string) error { return nil }
func (t *fakeTx) Commit(context.Context) error                             { return nil }
func (t *fakeTx) Rollback(context.Context) error                           { return nil }
func (t *fakeTx) Close()                                                   {}

func (f *fakeRepo) BeginSave(context.Context, string) (service.SaveTx, error) {
	if f.fail {
		return nil, errFakeRepo
	}
	if f.tx == nil {
		f.tx = &fakeTx{}
	}
	return f.tx, nil
}

var errFakeRepo = errors.New("repo down")

func (f *fakeRepo) FindNodeFields(context.Context, string, *int64) ([]model.TransactionRuleField, error) {
	if f.fail {
		return nil, errFakeRepo
	}
	return nil, nil
}

func (f *fakeRepo) FindSettingsVersion(context.Context, string) (int64, error) {
	if f.fail {
		return 0, errFakeRepo
	}
	return 1, nil
}

func (f *fakeRepo) FindExportFields(context.Context, string, *int64) ([]model.TransactionRuleField, error) {
	return nil, nil
}

func (f *fakeRepo) FindExportOverrides(context.Context, string, *int64) ([]model.TransactionRuleOverride, error) {
	return nil, nil
}

package service

import (
	"context"
	"errors"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

var errBoom = errors.New("boom")

// fakeOvTx records the writes of one override create or delete.
type fakeOvTx struct {
	inserted    []model.TransactionRuleOverride
	deleted     []int64
	deleteN     int64
	bumps       int
	committed   bool
	rolledBack  bool
	closed      bool
	insertErr   error
	deleteErr   error
	bumpErr     error
	commitErr   error
	rollbackErr error
}

func (t *fakeOvTx) InsertOverrides(_ context.Context, rows []model.TransactionRuleOverride) error {
	t.inserted = append(t.inserted, rows...)
	return t.insertErr
}

func (t *fakeOvTx) DeleteOverrides(_ context.Context, ids []int64) (int64, error) {
	t.deleted = append(t.deleted, ids...)
	return t.deleteN, t.deleteErr
}

func (t *fakeOvTx) BumpSettingsVersion(context.Context, int64, string) error {
	t.bumps++
	return t.bumpErr
}
func (t *fakeOvTx) Commit(context.Context) error   { t.committed = true; return t.commitErr }
func (t *fakeOvTx) Rollback(context.Context) error { t.rolledBack = true; return t.rollbackErr }
func (t *fakeOvTx) Close()                         { t.closed = true }

// fakeOverrideRepo is a SettingsRepository and OverrideRepository over in-memory answers.
type fakeOverrideRepo struct {
	*fakeRepo
	records    []dto.OverrideListRecord
	total      int
	listErr    error
	lastQuery  dto.OverrideQuery
	opts       OverrideOptionValues
	optsErr    error
	byID       []model.TransactionRuleOverride
	byIDErr    error
	forTargets []model.TransactionRuleOverride
	forErr     error
	dups       [][]int64 // one answer per FindDuplicateTargetIDs call; the last repeats
	dupCalls   int
	dupErr     error
	otx        *fakeOvTx
	beginErr   error
}

func newOverrideRepo() *fakeOverrideRepo {
	return &fakeOverrideRepo{fakeRepo: &fakeRepo{}, otx: &fakeOvTx{deleteN: 1}}
}

func (f *fakeOverrideRepo) ListOverrides(_ context.Context, _ string, q dto.OverrideQuery) ([]dto.OverrideListRecord, int, error) {
	f.lastQuery = q
	return f.records, f.total, f.listErr
}

func (f *fakeOverrideRepo) FindOverrideOptions(_ context.Context, _ string, q dto.OverrideQuery) (OverrideOptionValues, error) {
	f.lastQuery = q
	return f.opts, f.optsErr
}

func (f *fakeOverrideRepo) FindOverridesByIDs(_ context.Context, _ string, ids []int64) ([]model.TransactionRuleOverride, error) {
	var out []model.TransactionRuleOverride
	for _, o := range f.byID {
		for _, id := range ids {
			if o.ID == id {
				out = append(out, o)
			}
		}
	}
	return out, f.byIDErr
}

func (f *fakeOverrideRepo) FindOverridesForTargets(context.Context, string, string, []int64) ([]model.TransactionRuleOverride, error) {
	return f.forTargets, f.forErr
}

func (f *fakeOverrideRepo) FindDuplicateTargetIDs(context.Context, string, string, string, *int64, []int64) ([]int64, error) {
	f.dupCalls++
	if f.dupErr != nil {
		return nil, f.dupErr
	}
	if len(f.dups) == 0 {
		return nil, nil
	}
	i := f.dupCalls - 1
	if i >= len(f.dups) {
		i = len(f.dups) - 1
	}
	return f.dups[i], nil
}

func (f *fakeOverrideRepo) BeginOverrideWrite(context.Context, string) (OverrideTx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.otx, nil
}

// fakeTargets answers ResolveTargets from a fixed list (ids not listed are absent).
type fakeTargets struct {
	list []OverrideTarget
	err  error
	last string
}

func (f *fakeTargets) ResolveTargets(_ context.Context, _, scope string, ids []int64) ([]OverrideTarget, error) {
	f.last = scope
	var out []OverrideTarget
	for _, t := range f.list {
		for _, id := range ids {
			if t.ID == id {
				out = append(out, t)
			}
		}
	}
	return out, f.err
}

// fakeGroups answers the Group tree questions.
type fakeGroups struct {
	expanded  []int64
	current   map[int64]int64
	expandErr error
	currErr   error
}

func (f *fakeGroups) ExpandGroups(context.Context, string, []int64) ([]int64, error) {
	return f.expanded, f.expandErr
}

func (f *fakeGroups) CurrentAssetGroups(context.Context, string, []int64) (map[int64]int64, error) {
	return f.current, f.currErr
}

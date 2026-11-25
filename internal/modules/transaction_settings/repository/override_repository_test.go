package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreDto "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/dto"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

var errDB = errors.New("db down")

// overrideServicer answers the override reads by destination type and records the writes.
type overrideServicer struct {
	*writeServicer
	page      []coreDto.PaginationResult[dto.OverrideListRecord]
	options   map[string][]dto.OptionRecord // by a marker found in the query
	rows      []model.TransactionRuleOverride
	dups      []dto.DuplicateRecord
	queries   []string
	selectErr error
	insertErr error
	deleteN   int64
	deleteErr error
	inserted  any
	filter    sql_query.WhereQuery
}

func (o *overrideServicer) SelectMany(v any, _ context.Context, query string, _ ...any) error {
	o.queries = append(o.queries, query)
	if o.selectErr != nil {
		return o.selectErr
	}
	switch dst := v.(type) {
	case *[]coreDto.PaginationResult[dto.OverrideListRecord]:
		*dst = o.page
	case *[]dto.OptionRecord:
		for marker, recs := range o.options {
			if containsText(query, marker) {
				*dst = recs
			}
		}
	case *[]model.TransactionRuleOverride:
		*dst = o.rows
	case *[]dto.DuplicateRecord:
		*dst = o.dups
	default:
		return errors.New("unexpected destination")
	}
	return nil
}

func containsText(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func (o *overrideServicer) InsertManyWithData(_ context.Context, _ string, body any, _ ...coreService.ReturningConfig) (any, error) {
	o.inserted = body
	return nil, o.insertErr
}

func (o *overrideServicer) DeleteManyWithFilter(_ context.Context, _ string, f sql_query.WhereQuery) (int64, error) {
	o.filter = f
	return o.deleteN, o.deleteErr
}

func (o *overrideServicer) repo() *TransactionSettingsRepository {
	return NewTransactionSettingsRepository(func(string) coreService.PostgreSQLServicer { return o })
}

func newOverrideServicer() *overrideServicer {
	return &overrideServicer{writeServicer: &writeServicer{}}
}

func TestListOverrides(t *testing.T) {
	o := newOverrideServicer()
	o.page = []coreDto.PaginationResult[dto.OverrideListRecord]{{TotalRecords: 3, Data: []dto.OverrideListRecord{{ID: "1"}}}}
	rows, total, err := o.repo().ListOverrides(context.Background(), "c1", dto.OverrideQuery{Transaction: "borrow", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, "1", rows[0].ID)
	require.True(t, o.closed)

	o.page = nil
	rows, total, err = o.repo().ListOverrides(context.Background(), "c1", dto.OverrideQuery{Transaction: "borrow", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, total)

	o.selectErr = errDB
	_, _, err = o.repo().ListOverrides(context.Background(), "c1", dto.OverrideQuery{Transaction: "borrow", Page: 1, Limit: 10})
	require.ErrorIs(t, err, errDB)
}

func TestFindOverrideOptions(t *testing.T) {
	o := newOverrideServicer()
	o.options = map[string][]dto.OptionRecord{
		"o.scope":           {{Value: "group"}},
		"jsonb_object_keys": {{Value: "max_duration"}},
		"o.created_by_name": {{Value: "Rina"}},
	}
	vals, err := o.repo().FindOverrideOptions(context.Background(), "c1", dto.OverrideQuery{Transaction: "borrow", WantScope: true, WantOverridden: true, WantCreatedBy: true})
	require.NoError(t, err)
	require.Equal(t, service.OverrideOptionValues{Scopes: []string{"group"}, FieldKeys: []string{"max_duration"}, CreatedBy: []string{"Rina"}}, vals)
	require.Len(t, o.queries, 3)

	o = newOverrideServicer()
	vals, err = o.repo().FindOverrideOptions(context.Background(), "c1", dto.OverrideQuery{Transaction: "borrow"})
	require.NoError(t, err)
	require.Empty(t, vals.Scopes)
	require.Empty(t, o.queries)

	for _, q := range []dto.OverrideQuery{
		{Transaction: "borrow", WantScope: true}, {Transaction: "borrow", WantOverridden: true}, {Transaction: "borrow", WantCreatedBy: true},
	} {
		o.selectErr = errDB
		_, err = o.repo().FindOverrideOptions(context.Background(), "c1", q)
		require.ErrorIs(t, err, errDB)
	}
}

func TestOverrideLookups(t *testing.T) {
	o := newOverrideServicer()
	o.rows = []model.TransactionRuleOverride{{ID: 1}}
	rows, err := o.repo().FindOverridesByIDs(context.Background(), "c1", []int64{1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = o.repo().FindOverridesForTargets(context.Background(), "c1", "assign", []int64{10})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	o.dups = []dto.DuplicateRecord{{TargetID: 10}, {TargetID: 12}}
	ids, err := o.repo().FindDuplicateTargetIDs(context.Background(), "c1", "group", "borrow", nil, []int64{10, 11, 12})
	require.NoError(t, err)
	require.Equal(t, []int64{10, 12}, ids)

	o.selectErr = errDB
	_, err = o.repo().FindOverridesByIDs(context.Background(), "c1", []int64{1})
	require.ErrorIs(t, err, errDB)
	_, err = o.repo().FindOverridesForTargets(context.Background(), "c1", "assign", []int64{10})
	require.ErrorIs(t, err, errDB)
	_, err = o.repo().FindDuplicateTargetIDs(context.Background(), "c1", "group", "borrow", nil, []int64{10})
	require.ErrorIs(t, err, errDB)
}

func TestBeginOverrideWrite(t *testing.T) {
	o := newOverrideServicer()
	tx, err := o.repo().BeginOverrideWrite(context.Background(), "c1")
	require.NoError(t, err)
	require.True(t, o.txSet)

	require.NoError(t, tx.InsertOverrides(context.Background(), []model.TransactionRuleOverride{{TargetID: 10}}))
	require.Len(t, o.inserted, 1)

	o.deleteN = 2
	n, err := tx.DeleteOverrides(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	require.Contains(t, o.filter, "id")

	o.updated = 1
	require.NoError(t, tx.BumpSettingsVersion(context.Background(), 7, "Rina"))
	require.NoError(t, tx.Commit(context.Background()))
	require.True(t, o.committed)
	tx.Close()
	require.True(t, o.closed)
}

func TestOverrideWriteErrors(t *testing.T) {
	o := newOverrideServicer()
	tx, err := o.repo().BeginOverrideWrite(context.Background(), "c1")
	require.NoError(t, err)

	o.insertErr = &pgconn.PgError{Code: "23505"}
	require.ErrorIs(t, tx.InsertOverrides(context.Background(), nil), service.ErrDuplicateOverride)
	o.insertErr = &pgconn.PgError{Code: "23503"}
	err = tx.InsertOverrides(context.Background(), nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, service.ErrDuplicateOverride)
	o.insertErr = errDB
	require.ErrorIs(t, tx.InsertOverrides(context.Background(), nil), errDB)

	o.deleteErr = errDB
	_, err = tx.DeleteOverrides(context.Background(), []int64{1})
	require.ErrorIs(t, err, errDB)

	failing := &writeServicer{pool: fakePool{err: errDB}}
	_, err = NewTransactionSettingsRepository(func(string) coreService.PostgreSQLServicer { return failing }).BeginOverrideWrite(context.Background(), "c1")
	require.ErrorIs(t, err, errDB)
}

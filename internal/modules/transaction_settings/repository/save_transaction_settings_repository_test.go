package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

type fakePool struct {
	coreService.PgxPoolInterface
	err error
}

func (p fakePool) Begin(context.Context) (pgx.Tx, error) { return nil, p.err }

// writeServicer records the writes of a save transaction.
type writeServicer struct {
	coreService.PostgreSQLServicer
	pool       fakePool
	txSet      bool
	upsertBody any
	upsertConf string
	upsertCols []string
	upsertErr  error
	updateErr  error
	updated    int64
	updateSQL  string
	inserted   any
	insertErr  error
	committed  bool
	rolledBack bool
	closed     bool
	commitErr  error
}

func (w *writeServicer) GetPool() coreService.PgxPoolInterface { return w.pool }
func (w *writeServicer) SetTransaction(pgx.Tx)                 { w.txSet = true }
func (w *writeServicer) Close()                                { w.closed = true }
func (w *writeServicer) CommitTransaction(context.Context) error {
	w.committed = true
	return w.commitErr
}
func (w *writeServicer) RollbackTransaction(context.Context) error { w.rolledBack = true; return nil }

func (w *writeServicer) UpsertManyWithData(_ context.Context, _ string, body any, conflict string, cols []string, _ ...coreService.ReturningConfig) (any, error) {
	w.upsertBody, w.upsertConf, w.upsertCols = body, conflict, cols
	return nil, w.upsertErr
}

func (w *writeServicer) UpdateMany(_ context.Context, query string, _ ...any) (int64, error) {
	w.updateSQL = query
	return w.updated, w.updateErr
}

func (w *writeServicer) InsertOneWithData(_ context.Context, _ string, body any, _ ...coreService.ReturningConfig) (any, error) {
	w.inserted = body
	return nil, w.insertErr
}

func (w *writeServicer) repo() *TransactionSettingsRepository {
	return NewTransactionSettingsRepository(func(string) coreService.PostgreSQLServicer { return w })
}

func TestBeginSave(t *testing.T) {
	w := &writeServicer{}
	tx, err := w.repo().BeginSave(context.Background(), "c1")
	require.NoError(t, err)
	require.True(t, w.txSet)
	require.NoError(t, tx.Commit(context.Background()))
	require.True(t, w.committed)
	require.NoError(t, tx.Rollback(context.Background()))
	require.True(t, w.rolledBack)
	tx.Close()
	require.True(t, w.closed)
}

func TestBeginSave_BeginFails(t *testing.T) {
	w := &writeServicer{pool: fakePool{err: errors.New("no pool")}}
	_, err := w.repo().BeginSave(context.Background(), "c1")
	require.ErrorContains(t, err, "begin settings save transaction")
	require.True(t, w.closed)
}

func TestUpsertFields(t *testing.T) {
	w := &writeServicer{}
	tx, err := w.repo().BeginSave(context.Background(), "c1")
	require.NoError(t, err)
	require.NoError(t, tx.UpsertFields(context.Background(), nil))
	require.Nil(t, w.upsertBody)

	rows := []model.TransactionRuleField{{FieldKey: "max_duration"}}
	require.NoError(t, tx.UpsertFields(context.Background(), rows))
	require.Equal(t, rows, w.upsertBody)
	require.Equal(t, "(category_id, transaction_type, field_key)", w.upsertConf)
	require.Contains(t, w.upsertCols, "value")

	w.upsertErr = errors.New("db")
	require.ErrorContains(t, tx.UpsertFields(context.Background(), rows), "upsert rule fields")
}

func TestBumpSettingsVersion(t *testing.T) {
	t.Run("updates the existing row", func(t *testing.T) {
		w := &writeServicer{updated: 1}
		tx, _ := w.repo().BeginSave(context.Background(), "c1")
		require.NoError(t, tx.BumpSettingsVersion(context.Background(), 7, "Rina"))
		require.Contains(t, w.updateSQL, "settings_version + 1")
		require.Nil(t, w.inserted)
	})
	t.Run("creates the missing singleton at version 2", func(t *testing.T) {
		w := &writeServicer{}
		tx, _ := w.repo().BeginSave(context.Background(), "c1")
		require.NoError(t, tx.BumpSettingsVersion(context.Background(), 7, "Rina"))
		row, ok := w.inserted.(model.TransactionSettings)
		require.True(t, ok)
		require.Equal(t, int64(1), row.ID)
		require.Equal(t, int64(2), row.SettingsVersion)
		require.Equal(t, "Rina", *row.ModifiedByName)
	})
	t.Run("update fails", func(t *testing.T) {
		w := &writeServicer{updateErr: errors.New("db")}
		tx, _ := w.repo().BeginSave(context.Background(), "c1")
		require.ErrorContains(t, tx.BumpSettingsVersion(context.Background(), 7, "Rina"), "bump settings version")
	})
	t.Run("insert fails", func(t *testing.T) {
		w := &writeServicer{insertErr: errors.New("db")}
		tx, _ := w.repo().BeginSave(context.Background(), "c1")
		require.ErrorContains(t, tx.BumpSettingsVersion(context.Background(), 7, "Rina"), "insert settings version")
	})
}

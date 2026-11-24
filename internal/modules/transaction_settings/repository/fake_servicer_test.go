package repository

import (
	"context"
	"errors"
	"strings"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

// fakeServicer answers SelectMany/SelectOne by the table named in the query and records the queries.
type fakeServicer struct {
	coreService.PostgreSQLServicer
	fields    [][]model.TransactionRuleField // consumed one result per fields query
	overrides [][]model.TransactionRuleOverride
	version   any // SettingsVersionRow-compatible int64, or an error
	queries   []string
	closed    bool
	err       error
}

func (f *fakeServicer) Close() { f.closed = true }

func (f *fakeServicer) SelectMany(v any, _ context.Context, query string, _ ...any) error {
	f.queries = append(f.queries, query)
	if f.err != nil {
		return f.err
	}
	switch dst := v.(type) {
	case *[]model.TransactionRuleField:
		if len(f.fields) == 0 {
			return errors.New("unexpected fields query")
		}
		*dst, f.fields = f.fields[0], f.fields[1:]
	case *[]model.TransactionRuleOverride:
		if len(f.overrides) == 0 {
			return errors.New("unexpected overrides query")
		}
		*dst, f.overrides = f.overrides[0], f.overrides[1:]
	default:
		return errors.New("unexpected destination")
	}
	return nil
}

func (f *fakeServicer) SelectOne(v any, _ context.Context, query string, _ ...any) error {
	f.queries = append(f.queries, query)
	if err, ok := f.version.(error); ok {
		return err
	}
	if !strings.Contains(strings.ToLower(query), "settings_version") {
		return errors.New("unexpected query")
	}
	row, ok := v.(*query_builder.SettingsVersionRow)
	if !ok {
		return errors.New("unexpected destination")
	}
	row.SettingsVersion = f.version.(int64)
	return nil
}

func (f *fakeServicer) factory() ServicerFactory {
	return func(string) coreService.PostgreSQLServicer { return f }
}

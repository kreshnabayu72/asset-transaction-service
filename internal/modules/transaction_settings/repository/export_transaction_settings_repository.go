package repository

import (
	"context"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

// FindExportFields returns the rule fields an export lists: every stored row when categoryID is
// nil, otherwise the Company default rows followed by the rows of that Category.
func (r *TransactionSettingsRepository) FindExportFields(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleField, error) {
	svc := r.open(clientID)

	var out []model.TransactionRuleField
	if categoryID != nil {
		company, err := selectFields(ctx, svc, func() (string, []any, error) { return query_builder.BuildNodeFieldsQuery(nil) })
		if err != nil {
			return nil, err
		}
		out = company
	}
	rows, err := selectFields(ctx, svc, func() (string, []any, error) { return query_builder.BuildExportFieldsQuery(categoryID) })
	if err != nil {
		return nil, err
	}
	return append(out, rows...), nil
}

// FindExportOverrides returns the overrides an export lists, newest first: every override when
// categoryID is nil, otherwise those of that Category followed by the All categories overrides.
func (r *TransactionSettingsRepository) FindExportOverrides(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleOverride, error) {
	svc := r.open(clientID)

	var out []model.TransactionRuleOverride
	query, args, err := query_builder.BuildExportOverridesQuery(categoryID)
	if err != nil {
		return nil, fmt.Errorf("build export overrides query: %w", err)
	}
	if err := svc.SelectMany(&out, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select export overrides: %w", err)
	}
	if categoryID == nil {
		return out, nil
	}
	query, args, err = query_builder.BuildAllCategoriesOverridesQuery()
	if err != nil {
		return nil, fmt.Errorf("build all categories overrides query: %w", err)
	}
	var shared []model.TransactionRuleOverride
	if err := svc.SelectMany(&shared, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select all categories overrides: %w", err)
	}
	return append(out, shared...), nil
}

func selectFields(ctx context.Context, svc coreService.PostgreSQLServicer, build func() (string, []any, error)) ([]model.TransactionRuleField, error) {
	query, args, err := build()
	if err != nil {
		return nil, fmt.Errorf("build export fields query: %w", err)
	}
	var rows []model.TransactionRuleField
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select export fields: %w", err)
	}
	return rows, nil
}

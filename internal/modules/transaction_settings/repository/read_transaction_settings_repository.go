package repository

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/query_builder"
	"github.com/jackc/pgx/v5"
)

// InitialSettingsVersion is the version of an entity that never saved a change.
const InitialSettingsVersion int64 = 1

// FindNodeFields returns the stored rule fields of one node: the Company default rows when
// categoryID is nil, the rows of that Category otherwise.
func (r *TransactionSettingsRepository) FindNodeFields(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleField, error) {
	query, args, err := query_builder.BuildNodeFieldsQuery(categoryID)
	if err != nil {
		return nil, fmt.Errorf("build node fields query: %w", err)
	}
	svc := r.open(clientID)

	var rows []model.TransactionRuleField
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select node fields: %w", err)
	}
	return rows, nil
}

// FindSettingsVersion returns settings_version of the singleton; 1 when the row is absent.
func (r *TransactionSettingsRepository) FindSettingsVersion(ctx context.Context, clientID string) (int64, error) {
	query, args, err := query_builder.BuildSettingsVersionQuery()
	if err != nil {
		return 0, fmt.Errorf("build settings version query: %w", err)
	}
	svc := r.open(clientID)

	var row query_builder.SettingsVersionRow
	if err := svc.SelectOne(&row, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InitialSettingsVersion, nil
		}
		return 0, fmt.Errorf("select settings version: %w", err)
	}
	return row.SettingsVersion, nil
}

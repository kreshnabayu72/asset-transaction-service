package provider

import (
	"context"
	"fmt"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// CategoryDirectory answers Category lookups of the settings page from master-asset-registry.
type CategoryDirectory struct {
	mar marClient
}

// LookupCategory reports whether the Category exists, its full path and whether it is a leaf. An unknown,
// deleted or other-entity id answers Exists false (MAR answers found=false for all three).
func (d CategoryDirectory) LookupCategory(ctx context.Context, clientID string, id int64) (service.CategoryInfo, error) {
	items, err := d.mar.categories(ctx, clientID, []int64{id})
	if err != nil {
		return service.CategoryInfo{}, fmt.Errorf("lookup category: %w", err)
	}
	byID, err := indexByID(items, func(item marhttp.CategoryLookupItem) string { return item.ID })
	if err != nil {
		return service.CategoryInfo{}, fmt.Errorf("lookup category: %w", err)
	}
	item, ok := byID[id]
	if !ok || !item.Found {
		return service.CategoryInfo{}, nil
	}
	return service.CategoryInfo{Exists: true, Path: derefString(item.FullPath), Leaf: derefBool(item.IsLeaf)}, nil
}

// ListLeafCategories lists every live leaf Category of the client with its full path.
func (d CategoryDirectory) ListLeafCategories(ctx context.Context, clientID string) ([]service.LeafCategory, error) {
	items, err := d.mar.leafCategories(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("list leaf categories: %w", err)
	}
	out := make([]service.LeafCategory, 0, len(items))
	for _, item := range items {
		id, err := parseID(item.ID)
		if err != nil {
			return nil, fmt.Errorf("list leaf categories: %w", err)
		}
		out = append(out, service.LeafCategory{ID: id, Path: derefString(item.FullPath)})
	}
	return out, nil
}

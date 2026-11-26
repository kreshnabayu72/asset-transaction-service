package provider

import (
	"context"
	"fmt"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// AssetDirectory resolves the assets of effective rules: master-asset-registry gives the asset, its Asset Name
// and Category, iam-service the Group chain of the asset's Group.
type AssetDirectory struct {
	mar marClient
	iam iamClient
}

// ResolveAssets answers the assets in request order. A deleted, disposed, unknown or foreign asset fails the
// whole request with service.ErrAssetNotFound.
func (d AssetDirectory) ResolveAssets(ctx context.Context, clientID string, ids []int64) ([]service.AssetContext, error) {
	items, err := d.mar.assets(ctx, clientID, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve assets: %w", err)
	}
	byID, err := indexByID(items, func(item marhttp.AssetResolveItem) string { return item.ID })
	if err != nil {
		return nil, fmt.Errorf("resolve assets: %w", err)
	}
	found := make([]marhttp.AssetResolveItem, len(ids))
	for i, id := range ids {
		item, ok := byID[id]
		if !ok || !item.Found {
			return nil, service.ErrAssetNotFound
		}
		found[i] = item
	}
	if len(found) == 0 {
		return nil, nil
	}
	tree, err := loadGroups(ctx, d.iam, clientID)
	if err != nil {
		return nil, fmt.Errorf("resolve assets: %w", err)
	}
	out := make([]service.AssetContext, 0, len(found))
	for i, item := range found {
		asset, err := assetContext(ids[i], item, tree)
		if err != nil {
			return nil, err
		}
		out = append(out, asset)
	}
	return out, nil
}

// assetContext maps one resolved asset. The Category counts only while it is a leaf: a parent Category's values
// are not applied to the asset, so its id is left nil.
func assetContext(id int64, item marhttp.AssetResolveItem, tree groupTree) (service.AssetContext, error) {
	nameID, err := parseIDPtr(item.AssetNameID)
	if err != nil {
		return service.AssetContext{}, err
	}
	if nameID == nil {
		return service.AssetContext{}, fmt.Errorf("%w: asset %d has no Asset Name", errContract, id)
	}
	asset := service.AssetContext{
		ID:           id,
		Label:        derefString(item.DisplayName),
		CategoryPath: derefString(item.CategoryPath),
		AssetNameID:  *nameID,
		AssetName:    derefString(item.AssetNameName),
	}
	if derefBool(item.CategoryIsLeaf) {
		categoryID, err := parseIDPtr(item.CategoryID)
		if err != nil {
			return service.AssetContext{}, err
		}
		asset.CategoryID = categoryID
	}
	if item.GroupID != nil {
		groupID, err := parseID(*item.GroupID)
		if err != nil {
			return service.AssetContext{}, err
		}
		asset.Groups = tree.refs(tree.chain(groupID))
	}
	return asset, nil
}

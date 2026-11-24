package provider

import (
	"context"
	"fmt"
	"net/http"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"
)

const (
	pathCategoryLookup  = "/internal/categories/lookup"
	pathAssetResolve    = "/internal/assets/resolve"
	pathAssetNameLookup = "/internal/asset-names/lookup"
)

// marClient calls the internal lookups of master-asset-registry. Unknown, deleted, disposed and foreign ids
// answer Found false, so the module never tells them apart.
type marClient struct {
	client internalClient
}

// categories looks Categories up, one request per chunk of ids.
func (m marClient) categories(ctx context.Context, clientID string, ids []int64) ([]marhttp.CategoryLookupItem, error) {
	var items []marhttp.CategoryLookupItem
	for _, chunk := range chunkIDs(ids) {
		raw, err := m.client.call(ctx, http.MethodPost, pathCategoryLookup, clientID, nil,
			marhttp.CategoryLookupRequest{CategoryIDs: idStrings(chunk)})
		if err != nil {
			return nil, err
		}
		var data marhttp.CategoryLookupData
		if err := decodeData(raw, &data); err != nil {
			return nil, fmt.Errorf("decode %s: %w", pathCategoryLookup, err)
		}
		items = append(items, data.Results...)
	}
	return items, nil
}

// assets resolves Assets for effective rules and override targets, one request per chunk of ids.
func (m marClient) assets(ctx context.Context, clientID string, ids []int64) ([]marhttp.AssetResolveItem, error) {
	var items []marhttp.AssetResolveItem
	for _, chunk := range chunkIDs(ids) {
		raw, err := m.client.call(ctx, http.MethodPost, pathAssetResolve, clientID, nil,
			marhttp.AssetResolveRequest{AssetIDs: idStrings(chunk)})
		if err != nil {
			return nil, err
		}
		var data marhttp.AssetResolveData
		if err := decodeData(raw, &data); err != nil {
			return nil, fmt.Errorf("decode %s: %w", pathAssetResolve, err)
		}
		items = append(items, data.Results...)
	}
	return items, nil
}

// assetNames looks Asset Names up, one request per chunk of ids.
func (m marClient) assetNames(ctx context.Context, clientID string, ids []int64) ([]marhttp.AssetNameLookupItem, error) {
	var items []marhttp.AssetNameLookupItem
	for _, chunk := range chunkIDs(ids) {
		raw, err := m.client.call(ctx, http.MethodPost, pathAssetNameLookup, clientID, nil,
			marhttp.AssetNameLookupRequest{AssetNameIDs: idStrings(chunk)})
		if err != nil {
			return nil, err
		}
		var data marhttp.AssetNameLookupData
		if err := decodeData(raw, &data); err != nil {
			return nil, fmt.Errorf("decode %s: %w", pathAssetNameLookup, err)
		}
		items = append(items, data.Results...)
	}
	return items, nil
}

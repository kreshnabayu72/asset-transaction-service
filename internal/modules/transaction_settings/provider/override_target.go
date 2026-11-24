package provider

import (
	"context"
	"errors"
	"fmt"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// errUnknownScope reports an override scope the module does not define.
var errUnknownScope = errors.New("unknown override scope")

// OverrideTargetDirectory resolves override targets: Groups from enterprise-management, Asset Names and Assets
// from master-asset-registry. Ids that do not resolve are omitted, which the service reads as deleted.
type OverrideTargetDirectory struct {
	mar marClient
	ems emsClient
}

// ResolveTargets answers the targets of one scope that exist in the caller's entity.
func (d OverrideTargetDirectory) ResolveTargets(ctx context.Context, clientID, scope string, ids []int64) ([]service.OverrideTarget, error) {
	switch scope {
	case constants.ScopeGroup:
		return d.groupTargets(ctx, clientID, ids)
	case constants.ScopeAssetName:
		return d.assetNameTargets(ctx, clientID, ids)
	case constants.ScopeAsset:
		return d.assetTargets(ctx, clientID, ids)
	}
	return nil, fmt.Errorf("%w: %q", errUnknownScope, scope)
}

// groupTargets names a Group by its full path and carries its Group chain for the Group Role check.
func (d OverrideTargetDirectory) groupTargets(ctx context.Context, clientID string, ids []int64) ([]service.OverrideTarget, error) {
	tree, err := loadGroups(ctx, d.ems, clientID)
	if err != nil {
		return nil, err
	}
	var out []service.OverrideTarget
	for _, id := range ids {
		if !tree.has(id) {
			continue
		}
		out = append(out, service.OverrideTarget{ID: id, Exists: true, Name: tree.fullPath(id), GroupChain: tree.chain(id)})
	}
	return out, nil
}

// assetNameTargets names an Asset Name; an Asset Name has no Group.
func (d OverrideTargetDirectory) assetNameTargets(ctx context.Context, clientID string, ids []int64) ([]service.OverrideTarget, error) {
	items, err := d.mar.assetNames(ctx, clientID, ids)
	if err != nil {
		return nil, err
	}
	byID, err := indexByID(items, func(item marhttp.AssetNameLookupItem) string { return item.ID })
	if err != nil {
		return nil, err
	}
	var out []service.OverrideTarget
	for _, id := range ids {
		item, ok := byID[id]
		if !ok || !item.Found {
			continue
		}
		out = append(out, service.OverrideTarget{ID: id, Exists: true, Name: derefString(item.Name)})
	}
	return out, nil
}

// assetTargets names an Asset "[Asset Name] - [Increment]" and carries its Group and the Group chain of that Group.
func (d OverrideTargetDirectory) assetTargets(ctx context.Context, clientID string, ids []int64) ([]service.OverrideTarget, error) {
	items, err := d.mar.assets(ctx, clientID, ids)
	if err != nil {
		return nil, err
	}
	byID, err := indexByID(items, func(item marhttp.AssetResolveItem) string { return item.ID })
	if err != nil {
		return nil, err
	}
	var tree groupTree
	if len(items) > 0 {
		if tree, err = loadGroups(ctx, d.ems, clientID); err != nil {
			return nil, err
		}
	}
	var out []service.OverrideTarget
	for _, id := range ids {
		item, ok := byID[id]
		if !ok || !item.Found {
			continue
		}
		target := service.OverrideTarget{ID: id, Exists: true, Name: derefString(item.DisplayName)}
		if item.GroupID != nil {
			groupID, err := parseID(*item.GroupID)
			if err != nil {
				return nil, err
			}
			target.GroupID = &groupID
			target.GroupChain = tree.chain(groupID)
		}
		out = append(out, target)
	}
	return out, nil
}

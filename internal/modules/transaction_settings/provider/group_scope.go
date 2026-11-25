package provider

import (
	"context"
	"fmt"
)

// GroupScopeDirectory answers the Group tree questions of the Group Role scope of override list and delete.
type GroupScopeDirectory struct {
	mar marClient
	ems emsClient
}

// ExpandGroups returns the given Groups and all their descendants.
func (d GroupScopeDirectory) ExpandGroups(ctx context.Context, clientID string, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	tree, err := loadGroups(ctx, d.ems, clientID)
	if err != nil {
		return nil, err
	}
	return tree.expand(groupIDs), nil
}

// CurrentAssetGroups returns the current Group of each asset that still exists; deleted, disposed and unknown
// assets are omitted.
func (d GroupScopeDirectory) CurrentAssetGroups(ctx context.Context, clientID string, assetIDs []int64) (map[int64]int64, error) {
	items, err := d.mar.assets(ctx, clientID, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("read asset groups: %w", err)
	}
	groups := make(map[int64]int64, len(items))
	for _, item := range items {
		if !item.Found || item.GroupID == nil {
			continue
		}
		id, err := parseID(item.ID)
		if err != nil {
			return nil, err
		}
		groupID, err := parseID(*item.GroupID)
		if err != nil {
			return nil, err
		}
		groups[id] = groupID
	}
	return groups, nil
}

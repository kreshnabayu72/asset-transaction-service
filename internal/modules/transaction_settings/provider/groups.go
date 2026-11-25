package provider

import (
	"context"
	"fmt"
	"slices"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// groupTree is the Group tree of the caller's entity as enterprise-management lists it. The zero value is an
// empty tree: every lookup misses.
type groupTree struct {
	items map[int64]emshttp.GroupItem
}

func newGroupTree(list []emshttp.GroupItem) groupTree {
	items := make(map[int64]emshttp.GroupItem, len(list))
	for _, item := range list {
		items[item.ID] = item
	}
	return groupTree{items: items}
}

// loadGroups reads the Group tree of the caller's entity.
func loadGroups(ctx context.Context, ems emsClient, clientID string) (groupTree, error) {
	list, err := ems.groups(ctx, clientID)
	if err != nil {
		return groupTree{}, fmt.Errorf("read groups: %w", err)
	}
	return newGroupTree(list), nil
}

// has reports whether the Group exists in the caller's entity.
func (t groupTree) has(id int64) bool {
	_, ok := t.items[id]
	return ok
}

// fullPath is the Group path as enterprise-management writes it ("Parent > Child").
func (t groupTree) fullPath(id int64) string {
	return t.items[id].FullPath
}

// chain returns the Group and its ancestors, nearest first. An id outside the tree yields nil; a parent loop
// stops at the first repeated Group.
func (t groupTree) chain(id int64) []int64 {
	var out []int64
	seen := make(map[int64]bool)
	current := id
	for {
		item, ok := t.items[current]
		if !ok || seen[current] {
			return out
		}
		seen[current] = true
		out = append(out, current)
		if item.ParentID == nil {
			return out
		}
		current = *item.ParentID
	}
}

// refs pairs each Group of a chain with its full path, the snapshot effective rules report for an asset's Groups.
func (t groupTree) refs(chain []int64) []service.GroupRef {
	if len(chain) == 0 {
		return nil
	}
	refs := make([]service.GroupRef, 0, len(chain))
	for _, id := range chain {
		refs = append(refs, service.GroupRef{ID: id, Path: t.fullPath(id)})
	}
	return refs
}

// expand returns the given Groups and every descendant, sorted. Ids outside the tree are kept as given.
func (t groupTree) expand(ids []int64) []int64 {
	children := make(map[int64][]int64)
	for _, item := range t.items {
		if item.ParentID != nil {
			children[*item.ParentID] = append(children[*item.ParentID], item.ID)
		}
	}
	seen := make(map[int64]bool)
	queue := slices.Clone(ids)
	var out []int64
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		queue = append(queue, children[id]...)
	}
	slices.Sort(out)
	return out
}

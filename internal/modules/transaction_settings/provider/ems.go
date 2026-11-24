package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
)

const (
	pathGroups              = "/internal/groups"
	pathSubscriptions       = "/internal/subscriptions"
	pathEffectiveCapacities = "/internal/plans-features/effective-capacities"
	// productFixedAsset is the product of the Fixed Asset subscription and of the effective-capacities query.
	productFixedAsset = "fixed_asset"
)

// emsClient calls the internal reads of enterprise-management.
type emsClient struct {
	client internalClient
}

// groups lists every Group of the caller's entity.
func (e emsClient) groups(ctx context.Context, clientID string) ([]emshttp.GroupItem, error) {
	raw, err := e.client.call(ctx, http.MethodGet, pathGroups, clientID, nil, nil)
	if err != nil {
		return nil, err
	}
	var items []emshttp.GroupItem
	if err := decodeData(raw, &items); err != nil {
		return nil, fmt.Errorf("decode %s: %w", pathGroups, err)
	}
	return items, nil
}

// subscriptions lists the subscriptions of the caller's entity with their plan feature rows.
func (e emsClient) subscriptions(ctx context.Context, clientID string) ([]emshttp.SubscriptionItem, error) {
	raw, err := e.client.call(ctx, http.MethodGet, pathSubscriptions, clientID, nil, nil)
	if err != nil {
		return nil, err
	}
	var data emshttp.SubscriptionsData
	if err := decodeData(raw, &data); err != nil {
		return nil, fmt.Errorf("decode %s: %w", pathSubscriptions, err)
	}
	return data.Subscriptions, nil
}

// effectiveCapacities reads the addon licence ledger of the Fixed Asset product.
func (e emsClient) effectiveCapacities(ctx context.Context, clientID string) (emshttp.EffectiveCapacitiesData, error) {
	raw, err := e.client.call(ctx, http.MethodGet, pathEffectiveCapacities, clientID,
		url.Values{"product": {productFixedAsset}}, nil)
	if err != nil {
		return emshttp.EffectiveCapacitiesData{}, err
	}
	var data emshttp.EffectiveCapacitiesData
	if err := decodeData(raw, &data); err != nil {
		return emshttp.EffectiveCapacitiesData{}, fmt.Errorf("decode %s: %w", pathEffectiveCapacities, err)
	}
	return data, nil
}

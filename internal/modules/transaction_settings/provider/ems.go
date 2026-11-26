package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
)

const (
	pathSubscriptions       = "/internal/subscriptions"
	pathEffectiveCapacities = "/internal/plans-features/effective-capacities"
	// productFixedAsset is the product of the Fixed Asset subscription and of the effective-capacities query.
	productFixedAsset = "fixed_asset"
)

// emsClient calls the internal reads of enterprise-management.
type emsClient struct {
	client internalClient
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

// pathClient is the enterprise-management client record of the caller's tenant; its entityType tells a network actor
// apart from a Client tenant (the same record iam-service reads for SD-4).
const pathClient = "/internal/client"

// EntityProvider reads the entity type of the caller's tenant for the transaction settings routes.
type EntityProvider struct {
	ems emsClient
}

// EntityType answers the entity type of the caller's tenant: "client" for a Client tenant.
func (p EntityProvider) EntityType(ctx context.Context, clientID string) (string, error) {
	return p.ems.entityType(ctx, clientID)
}

func (p EntityProvider) ClientStatus(ctx context.Context, clientID string) (string, error) {
	return p.ems.clientStatus(ctx, clientID)
}

// entityType reads the entity type of the caller's tenant from the enterprise-management client record.
func (e emsClient) entityType(ctx context.Context, clientID string) (string, error) {
	raw, err := e.client.call(ctx, http.MethodGet, pathClient, clientID, nil, nil)
	if err != nil {
		return "", err
	}
	var data struct {
		EntityType string `json:"entityType"`
	}
	if err := decodeData(raw, &data); err != nil {
		return "", fmt.Errorf("decode %s: %w", pathClient, err)
	}
	return data.EntityType, nil
}

func (e emsClient) clientStatus(ctx context.Context, clientID string) (string, error) {
	raw, err := e.client.call(ctx, http.MethodGet, pathClient, clientID, nil, nil)
	if err != nil {
		return "", err
	}
	var data struct {
		Status string `json:"status"`
	}
	if err := decodeData(raw, &data); err != nil {
		return "", fmt.Errorf("decode %s: %w", pathClient, err)
	}
	return data.Status, nil
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

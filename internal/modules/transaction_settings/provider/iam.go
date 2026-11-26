package provider

import (
	"context"
	"fmt"
	"net/http"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
	iamhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/iam/http"
)

const (
	pathApproverPresence = "/internal/transaction-roles/approver-presence"
	pathFixedAssetGroups = "/internal/fixed-asset-groups"
)

// iamClient calls the internal reads of iam-service: the Transaction Role Approver presence, the fixed-asset
// Group tree and the Group manager contact of a Group.
type iamClient struct {
	client internalClient
}

// approverPresence answers one boolean per requested transaction key.
func (i iamClient) approverPresence(ctx context.Context, clientID string, transactions []string) (iamhttp.ApproverPresenceResponse, error) {
	raw, err := i.client.call(ctx, http.MethodPost, pathApproverPresence, clientID, nil,
		iamhttp.ApproverPresenceRequest{Transactions: transactions})
	if err != nil {
		return nil, err
	}
	var presence iamhttp.ApproverPresenceResponse
	if err := decodeData(raw, &presence); err != nil {
		return nil, fmt.Errorf("decode %s: %w", pathApproverPresence, err)
	}
	return presence, nil
}

// managerContact answers the nearest active Group Manager of the Group walking up its ancestors, else Total Control.
func (i iamClient) managerContact(ctx context.Context, clientID string, groupID int64) (iamhttp.ManagerContactResponse, error) {
	path := fmt.Sprintf("/internal/fixed-asset-groups/%d/manager-contact", groupID)
	raw, err := i.client.call(ctx, http.MethodGet, path, clientID, nil, nil)
	if err != nil {
		return iamhttp.ManagerContactResponse{}, err
	}
	var contact iamhttp.ManagerContactResponse
	if err := decodeData(raw, &contact); err != nil {
		return iamhttp.ManagerContactResponse{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return contact, nil
}

// groups lists every fixed-asset Group of the caller's entity. IAM answers the _id, name, parentId and fullPath
// of enterprise-management's group item; the other fields stay zero.
func (i iamClient) groups(ctx context.Context, clientID string) ([]emshttp.GroupItem, error) {
	raw, err := i.client.call(ctx, http.MethodGet, pathFixedAssetGroups, clientID, nil, nil)
	if err != nil {
		return nil, err
	}
	var items []emshttp.GroupItem
	if err := decodeData(raw, &items); err != nil {
		return nil, fmt.Errorf("decode %s: %w", pathFixedAssetGroups, err)
	}
	return items, nil
}

package provider

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// errNoContactGroup reports a deny contact asked for an asset that has no Group.
var errNoContactGroup = errors.New("deny contact: asset has no Group")

// DenyContactResolver answers the Group Manager or Total Control contact of effective rules from iam-service.
type DenyContactResolver struct {
	iam iamClient
}

// ResolveDenyContact asks iam-service from the asset's own Group (the first id); iam-service walks up the ancestors.
func (d DenyContactResolver) ResolveDenyContact(ctx context.Context, clientID string, groupIDs []int64) (dto.DenyContact, error) {
	if len(groupIDs) == 0 {
		return dto.DenyContact{}, errNoContactGroup
	}
	contact, err := d.iam.managerContact(ctx, clientID, groupIDs[0])
	if err != nil {
		return dto.DenyContact{}, fmt.Errorf("read manager contact: %w", err)
	}
	return dto.DenyContact{Name: contact.Name, Role: contact.Role}, nil
}

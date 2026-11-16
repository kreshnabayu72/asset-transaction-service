package service

import (
	"context"
	"errors"
	"strconv"

	contract "gitea.tagsamurai.local/BETS-V2/service-contracts/services/asset_transaction/http"
)

// ErrInvalidGroupIDs maps to 422: empty list or a non-numeric id.
var ErrInvalidGroupIDs = errors.New("groupIds must be a non-empty list of numeric ids")

// Service answers the iam-called group delete guard (LLD guard 3).
type Service struct{}

func NewService() *Service { return &Service{} }

// GroupDeleteGuard returns one result per distinct submitted group id, in
// request order. ongoingTransfer / ongoingDisposal are true iff an in-flight
// (not completed/cancelled/rejected) transfer or disposal references the group.
//
// NOTE: this service is still a skeleton and has no transfer or disposal
// tables yet, so nothing can be in flight: every flag is false. Replace the
// constant false with a repository lookup once those tables exist.
func (s *Service) GroupDeleteGuard(_ context.Context, _ string, req contract.GroupDeleteGuardRequest) (*contract.GroupDeleteGuardData, error) {
	if len(req.GroupIDs) == 0 {
		return nil, ErrInvalidGroupIDs
	}
	seen := make(map[int64]bool, len(req.GroupIDs))
	out := &contract.GroupDeleteGuardData{Results: make([]contract.GroupDeleteGuardItem, 0, len(req.GroupIDs))}
	for _, raw := range req.GroupIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return nil, ErrInvalidGroupIDs
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out.Results = append(out.Results, contract.GroupDeleteGuardItem{GroupID: strconv.FormatInt(id, 10)})
	}
	return out, nil
}

package service

import (
	"context"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type MasterChangesRepository interface {
	ApplyMasterChanges(ctx context.Context, clientID string, change dto.MasterChangesRequest) (int64, error)
}

type MasterChangesService struct {
	repository MasterChangesRepository
}

func NewMasterChangesService(repository MasterChangesRepository) *MasterChangesService {
	return &MasterChangesService{repository: repository}
}

func (s *MasterChangesService) ApplyMasterChanges(
	ctx context.Context,
	clientID string,
	change dto.MasterChangesRequest,
) (int64, error) {
	if change.IsNoOp() {
		return 0, nil
	}
	return s.repository.ApplyMasterChanges(ctx, clientID, change)
}

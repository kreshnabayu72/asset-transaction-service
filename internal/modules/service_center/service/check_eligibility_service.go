package service

import (
	"context"
	"fmt"

	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type EligibilitySnapshot = dto.EligibilitySnapshot

type CheckEligibilityRepository interface {
	LoadEligibilitySnapshot(ctx context.Context, clientID string, serviceCenterID int64) (EligibilitySnapshot, bool, error)
}

type CheckEligibilityService struct {
	repository CheckEligibilityRepository
}

func NewCheckEligibilityService(repository CheckEligibilityRepository) *CheckEligibilityService {
	return &CheckEligibilityService{repository: repository}
}

func (s *CheckEligibilityService) CheckEligibility(ctx context.Context, clientID string, request dto.CheckEligibilityRequest) (dto.CheckEligibilityResponse, error) {
	snapshot, found, err := s.repository.LoadEligibilitySnapshot(ctx, clientID, request.ServiceCenterID)
	if err != nil {
		return dto.CheckEligibilityResponse{}, fmt.Errorf("load service center eligibility: %w", err)
	}
	if !found {
		return dto.CheckEligibilityResponse{}, coreEntity.NotFound("Service center not found")
	}

	if !snapshot.IsActive {
		return ineligible(snapshot.Name, "inactive"), nil
	}
	if !containsString(snapshot.ServiceTypes, request.ServiceType) {
		return ineligible(snapshot.Name, "service_type"), nil
	}
	for _, asset := range request.Assets {
		if !containsInt64(snapshot.GroupIDs, asset.GroupID) {
			return ineligible(snapshot.Name, "available_to"), nil
		}
	}
	if len(snapshot.CategoryIDs) > 0 {
		for _, asset := range request.Assets {
			if !containsInt64(snapshot.CategoryIDs, asset.CategoryID) {
				return ineligible(snapshot.Name, "specialization"), nil
			}
		}
	}
	if len(snapshot.BrandIDs) > 0 {
		for _, asset := range request.Assets {
			if asset.BrandID != nil && !containsInt64(snapshot.BrandIDs, *asset.BrandID) {
				return ineligible(snapshot.Name, "brands"), nil
			}
		}
	}

	return dto.CheckEligibilityResponse{Eligible: true}, nil
}

func ineligible(name, reason string) dto.CheckEligibilityResponse {
	message := fmt.Sprintf("%s doesn't cover this asset.", name)
	return dto.CheckEligibilityResponse{Eligible: false, Reason: &reason, Message: &message}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

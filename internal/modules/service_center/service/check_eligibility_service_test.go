package service

import (
	"context"
	"errors"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type eligibilityRepositoryFake struct {
	snapshot EligibilitySnapshot
	found    bool
	err      error
	calls    int
	clientID string
	centerID int64
}

func (r *eligibilityRepositoryFake) LoadEligibilitySnapshot(_ context.Context, clientID string, serviceCenterID int64) (EligibilitySnapshot, bool, error) {
	r.calls++
	r.clientID = clientID
	r.centerID = serviceCenterID
	return r.snapshot, r.found, r.err
}

func TestCheckEligibilityReturnsFirstFailureInContractOrder(t *testing.T) {
	tests := []struct {
		name     string
		snapshot EligibilitySnapshot
		request  dto.CheckEligibilityRequest
		want     string
	}{
		{
			name: "inactive precedes all other checks",
			snapshot: EligibilitySnapshot{
				Name:         "Partner Workshop",
				IsActive:     false,
				ServiceTypes: []string{"Routine"},
				GroupIDs:     []int64{10},
				CategoryIDs:  []int64{20},
				BrandIDs:     []int64{30},
			},
			request: dto.CheckEligibilityRequest{
				ServiceType: "Repair",
				Assets:      []dto.EligibilityAsset{{GroupID: 11, CategoryID: 21, BrandID: int64Pointer(31)}},
			},
			want: "inactive",
		},
		{
			name:     "service type precedes group",
			snapshot: EligibilitySnapshot{Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"}},
			request: dto.CheckEligibilityRequest{
				ServiceType: "Repair",
				Assets:      []dto.EligibilityAsset{{GroupID: 11, CategoryID: 21, BrandID: int64Pointer(31)}},
			},
			want: "service_type",
		},
		{
			name:     "group applies to every asset",
			snapshot: EligibilitySnapshot{Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"}, GroupIDs: []int64{10}},
			request: dto.CheckEligibilityRequest{
				ServiceType: "Routine",
				Assets:      []dto.EligibilityAsset{{GroupID: 10}, {GroupID: 11}},
			},
			want: "available_to",
		},
		{
			name: "category applies to every asset when configured",
			snapshot: EligibilitySnapshot{
				Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"},
				GroupIDs: []int64{10}, CategoryIDs: []int64{20},
			},
			request: dto.CheckEligibilityRequest{
				ServiceType: "Routine",
				Assets:      []dto.EligibilityAsset{{GroupID: 10, CategoryID: 20}, {GroupID: 10, CategoryID: 21}},
			},
			want: "specialization",
		},
		{
			name: "brand applies to non-null brands when configured",
			snapshot: EligibilitySnapshot{
				Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"},
				GroupIDs: []int64{10}, CategoryIDs: []int64{20}, BrandIDs: []int64{30},
			},
			request: dto.CheckEligibilityRequest{
				ServiceType: "Routine",
				Assets:      []dto.EligibilityAsset{{GroupID: 10, CategoryID: 20, BrandID: int64Pointer(30)}, {GroupID: 10, CategoryID: 20, BrandID: int64Pointer(31)}},
			},
			want: "brands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &eligibilityRepositoryFake{snapshot: tt.snapshot, found: true}
			service := NewCheckEligibilityService(repository)

			response, err := service.CheckEligibility(context.Background(), "client-123", tt.request)
			if err != nil {
				t.Fatalf("CheckEligibility() error = %v", err)
			}
			if response.Eligible || response.Reason == nil || *response.Reason != tt.want {
				t.Fatalf("CheckEligibility() response = %#v, want reason %q", response, tt.want)
			}
			if response.Message == nil || *response.Message != "Partner Workshop doesn't cover this asset." {
				t.Fatalf("CheckEligibility() message = %v", response.Message)
			}
			if repository.calls != 1 || repository.clientID != "client-123" {
				t.Fatalf("repository calls/client ID = %d/%q", repository.calls, repository.clientID)
			}
		})
	}
}

func TestCheckEligibilitySkipsUnconfiguredListsAndNullAssetBrands(t *testing.T) {
	tests := []struct {
		name     string
		snapshot EligibilitySnapshot
		assets   []dto.EligibilityAsset
	}{
		{
			name: "null asset brand skips configured brand links",
			snapshot: EligibilitySnapshot{
				Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"},
				GroupIDs: []int64{10}, BrandIDs: []int64{30},
			},
			assets: []dto.EligibilityAsset{
				{GroupID: 10, CategoryID: 20, BrandID: int64Pointer(30)},
				{GroupID: 10, CategoryID: 21, BrandID: nil},
			},
		},
		{
			name: "empty category and brand links allow all values",
			snapshot: EligibilitySnapshot{
				Name: "Partner Workshop", IsActive: true, ServiceTypes: []string{"Routine"},
				GroupIDs: []int64{10},
			},
			assets: []dto.EligibilityAsset{
				{GroupID: 10, CategoryID: 20, BrandID: int64Pointer(31)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &eligibilityRepositoryFake{found: true, snapshot: tt.snapshot}
			service := NewCheckEligibilityService(repository)
			request := dto.CheckEligibilityRequest{
				ServiceCenterID: 99,
				ServiceType:     "Routine",
				Assets:          tt.assets,
			}

			response, err := service.CheckEligibility(context.Background(), "client-123", request)
			if err != nil {
				t.Fatalf("CheckEligibility() error = %v", err)
			}
			if !response.Eligible || response.Reason != nil || response.Message != nil {
				t.Fatalf("CheckEligibility() response = %#v, want eligible with null reason/message", response)
			}
			if repository.centerID != request.ServiceCenterID {
				t.Fatalf("serviceCenterID = %d, want %d", repository.centerID, request.ServiceCenterID)
			}
		})
	}
}

func TestCheckEligibilityNotFoundAndRepositoryError(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repository := &eligibilityRepositoryFake{found: false}
		service := NewCheckEligibilityService(repository)
		_, err := service.CheckEligibility(context.Background(), "client-123", dto.CheckEligibilityRequest{ServiceCenterID: 99})
		if err == nil {
			t.Fatal("CheckEligibility() error = nil, want not found")
		}
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		cause := errors.New("database unavailable")
		repository := &eligibilityRepositoryFake{err: cause}
		service := NewCheckEligibilityService(repository)
		_, err := service.CheckEligibility(context.Background(), "client-123", dto.CheckEligibilityRequest{ServiceCenterID: 99})
		if !errors.Is(err, cause) {
			t.Fatalf("CheckEligibility() error = %v, want wrapped cause", err)
		}
	})
}

func int64Pointer(value int64) *int64 {
	return &value
}

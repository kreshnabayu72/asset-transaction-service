package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

func TestListItemPreservesKnownEmptyFacetsAndOmitsUnavailableCounts(t *testing.T) {
	item := listItem(query_builder.ReadCenterRow{
		ID:          1,
		Name:        "Center",
		GroupIDs:    []int64{},
		CategoryIDs: nil,
		BrandIDs:    []int64{},
	}, middleware.UserAccessCache{}, dto.ServiceCenterReadAccess{TotalControl: true}, ReferenceData{})
	body, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{`"availableTo":[]`, `"specializations":[]`, `"brands":[]`} {
		if !strings.Contains(text, expected) {
			t.Errorf("response missing known empty facet %s: %s", expected, text)
		}
	}
	for _, unavailable := range []string{`"usage"`, `"isInUse"`} {
		if strings.Contains(text, unavailable) {
			t.Errorf("response fabricated unavailable field %s: %s", unavailable, text)
		}
	}
}

func TestListItemOmitsUnresolvedReferenceNames(t *testing.T) {
	item := listItem(query_builder.ReadCenterRow{
		ID:          2,
		Name:        "Center",
		GroupIDs:    []int64{11},
		CategoryIDs: []int64{22},
		BrandIDs:    []int64{33},
	}, middleware.UserAccessCache{}, dto.ServiceCenterReadAccess{TotalControl: true}, ReferenceData{})
	body, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, unresolved := range []string{
		`"availableTo":[{"_id":"11","name":"","path":""}]`,
		`"specializations":[{"_id":"22","name":"","path":""}]`,
		`"brands":[{"_id":"33","name":"","isActive":false}]`,
	} {
		if !strings.Contains(text, unresolved) {
			t.Errorf("response should preserve stored ID without fabricating names %s: %s", unresolved, text)
		}
	}
}

type exportRowsOnlyRepository struct{}

func (exportRowsOnlyRepository) List(context.Context, string, dto.ListServiceCentersRequest, []string, dto.ServiceCenterReadAccess) ([]query_builder.ReadCenterRow, int, error) {
	return nil, 0, nil
}

func (exportRowsOnlyRepository) ExportRows(context.Context, string, dto.ExportServiceCentersRequest, []string, dto.ServiceCenterReadAccess) ([]query_builder.ReadCenterRow, error) {
	return nil, nil
}

func (exportRowsOnlyRepository) Get(context.Context, string, int64, dto.ServiceCenterReadAccess) (ReadServiceCenterDetail, bool, error) {
	return ReadServiceCenterDetail{}, false, nil
}

func (exportRowsOnlyRepository) Options(context.Context, string, map[string]bool, dto.ServiceCenterReadAccess) (map[string][]dto.OptionItem, error) {
	return nil, nil
}

func TestServiceCenterExportUsesUTF8BOMOnlyForCSV(t *testing.T) {
	service := NewServiceCenterReadService(exportRowsOnlyRepository{})
	columns := []dto.ExportColumn{{Key: "name", Header: "Name"}}

	for _, format := range []string{"csv", "xlsx"} {
		t.Run(format, func(t *testing.T) {
			result, err := service.Export(context.Background(), "tenant-a", dto.ExportServiceCentersRequest{
				Columns: columns,
				Format:  format,
			}, dto.ServiceCenterReadAccess{TotalControl: true})
			if err != nil {
				t.Fatal(err)
			}
			hasBOM := bytes.HasPrefix(result.Data, []byte{0xEF, 0xBB, 0xBF})
			if got, want := hasBOM, format == "csv"; got != want {
				t.Errorf("UTF-8 BOM present = %v, want %v", got, want)
			}
		})
	}
}

func TestHTTPServiceCenterReferenceResolverUsesOwnerRoutesAndFoundFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Service-Token") != "service-secret" || r.Header.Get("X-Client-ID") != "client-a" {
			http.Error(w, "missing service/client scope", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/internal/groups":
			_, _ = io.WriteString(w, `{"data":[{"_id":7,"name":"Default Group","fullPath":"Default Group"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/internal/brands/dropdown":
			_, _ = io.WriteString(w, `{"data":[{"_id":8,"name":"E2E Brand"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/internal/categories/lookup":
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "missing JSON content type", http.StatusBadRequest)
				return
			}
			var request struct {
				CategoryIDs []string `json:"categoryIds"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.Join(request.CategoryIDs, ",") != "9,10" {
				http.Error(w, "wrong category lookup body", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"results":[{"id":9,"found":true,"name":"Category Nine","fullPath":"Parent / Category Nine","isLeaf":true},{"id":10,"found":false,"name":null,"fullPath":null,"isLeaf":null}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resolver := NewHTTPServiceCenterReferenceResolver(server.URL, server.URL, "service-secret")
	resolved, err := resolver.Resolve(context.Background(), "client-a", []int64{7}, []int64{9, 10}, []int64{8})
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved.Groups[7]; got.Name != "Default Group" || got.Path != "Default Group" {
		t.Fatalf("unexpected group: %+v", got)
	}
	if got := resolved.Categories[9]; got.Name != "Category Nine" || got.Path != "Parent / Category Nine" {
		t.Fatalf("unexpected category: %+v", got)
	}
	if _, ok := resolved.Categories[10]; ok {
		t.Fatal("category with found=false must not be fabricated")
	}
	if got := resolved.Brands[8]; got.Name != "E2E Brand" || got.IsActive {
		t.Fatalf("brand dropdown must preserve its fields without fabricating active status: %+v", got)
	}
}

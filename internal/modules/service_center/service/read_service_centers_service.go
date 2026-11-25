package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
)

type ReadServiceCenterRepository interface {
	List(context.Context, string, dto.ListServiceCentersRequest, []string, dto.ServiceCenterReadAccess) ([]query_builder.ReadCenterRow, int, error)
	ExportRows(context.Context, string, dto.ExportServiceCentersRequest, []string, dto.ServiceCenterReadAccess) ([]query_builder.ReadCenterRow, error)
	Get(context.Context, string, int64, dto.ServiceCenterReadAccess) (ReadServiceCenterDetail, bool, error)
	Options(context.Context, string, map[string]bool, dto.ServiceCenterReadAccess) (map[string][]dto.OptionItem, error)
}

type ReadServiceCenterDetail struct {
	Center   query_builder.ReadCenterRow
	Contacts []query_builder.ContactPersonRow
}

type ServiceCenterReadService struct {
	repository ReadServiceCenterRepository
	references ServiceCenterReferenceResolver
}

func NewServiceCenterReadService(repo ReadServiceCenterRepository, references ...ServiceCenterReferenceResolver) *ServiceCenterReadService {
	var resolver ServiceCenterReferenceResolver
	if len(references) > 0 {
		resolver = references[0]
	}
	return &ServiceCenterReadService{repository: repo, references: resolver}
}

func (s *ServiceCenterReadService) List(
	ctx context.Context,
	clientID string,
	request dto.ListServiceCentersRequest,
	access dto.ServiceCenterReadAccess,
	roles middleware.UserAccessCache,
) ([]dto.ListServiceCenterItem, int, error) {
	rows, total, err := s.repository.List(ctx, clientID, request, request.VisibleColumns(), access)
	if err != nil {
		return nil, 0, err
	}
	references, err := s.resolveReferences(ctx, clientID, rows)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.ListServiceCenterItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, listItem(row, roles, access, references))
	}
	return items, total, nil
}

func (s *ServiceCenterReadService) Get(
	ctx context.Context,
	clientID string,
	id int64,
	access dto.ServiceCenterReadAccess,
	roles middleware.UserAccessCache,
) (dto.ServiceCenterDetail, bool, error) {
	result, found, err := s.repository.Get(ctx, clientID, id, access)
	if err != nil || !found {
		return dto.ServiceCenterDetail{}, found, err
	}
	row := result.Center
	references, err := s.resolveReferences(ctx, clientID, []query_builder.ReadCenterRow{row})
	if err != nil {
		return dto.ServiceCenterDetail{}, false, err
	}
	detail := dto.ServiceCenterDetail{
		ID:                row.ID,
		Name:              row.Name,
		IsActive:          row.IsActive,
		ServiceTypes:      row.ServiceTypes,
		AvailableTo:       resolvedHierarchyRefs(row.GroupIDs, references.Groups),
		Specializations:   resolvedHierarchyRefs(row.CategoryIDs, references.Categories),
		Brands:            resolvedBrandRefs(row.BrandIDs, references.Brands),
		Country:           row.Country,
		ProvinceState:     row.ProvinceState,
		City:              row.City,
		PostalCode:        row.PostalCode,
		AddressLine:       row.AddressLine,
		CompanyEmail:      row.CompanyEmail,
		ContactPersons:    make([]dto.ContactPerson, 0, len(result.Contacts)),
		Notes:             row.Notes,
		CreatedBy:         dto.NamedRef{ID: row.CreatedByID, Name: row.CreatedByName},
		CreatedAt:         row.CreatedAt,
		ModifiedBy:        modifiedBy(row.ModifiedByName),
		UpdatedAt:         row.UpdatedAt,
		Version:           row.Version,
		SystemPermissions: permissionsFor(row.GroupIDs, roles, access),
	}
	if row.CompanyPhoneDialCode != nil && row.CompanyPhoneNumber != nil {
		detail.CompanyPhone = &dto.Phone{DialCode: *row.CompanyPhoneDialCode, Number: *row.CompanyPhoneNumber}
	}
	for _, contact := range result.Contacts {
		person := dto.ContactPerson{Name: contact.Name, Email: contact.Email, IsPrimary: contact.IsPrimary}
		if contact.PhoneDialCode != nil && contact.PhoneNumber != nil {
			person.Phone = &dto.Phone{DialCode: *contact.PhoneDialCode, Number: *contact.PhoneNumber}
		}
		detail.ContactPersons = append(detail.ContactPersons, person)
	}
	return detail, true, nil
}

func (s *ServiceCenterReadService) Options(
	ctx context.Context,
	clientID string,
	facets map[string]bool,
	access dto.ServiceCenterReadAccess,
) (map[string][]dto.OptionItem, error) {
	result, err := s.repository.Options(ctx, clientID, facets, access)
	if err != nil {
		return nil, err
	}
	references, err := s.resolveReferences(ctx, clientID, []query_builder.ReadCenterRow{{
		GroupIDs:    optionIDs(result["availableToOptions"]),
		CategoryIDs: optionIDs(result["specializationOptions"]),
		BrandIDs:    optionIDs(result["brandOptions"]),
	}})
	if err != nil {
		return nil, err
	}
	allowedGroups := make(map[int64]struct{}, len(access.GroupIDs))
	for _, id := range access.GroupIDs {
		allowedGroups[id] = struct{}{}
	}
	for facet, items := range result {
		resolved := make([]dto.OptionItem, 0, len(items))
		for _, option := range items {
			id, parseErr := strconv.ParseInt(option.Value, 10, 64)
			if parseErr != nil || id <= 0 {
				continue
			}
			switch facet {
			case "availableToOptions":
				ref, ok := references.Groups[id]
				if !ok {
					continue
				}
				if !access.TotalControl && !access.ReadOnly {
					if _, allowed := allowedGroups[id]; !allowed {
						continue
					}
				}
				option.Label = hierarchyLabel(ref)
			case "specializationOptions":
				ref, ok := references.Categories[id]
				if !ok {
					continue
				}
				option.Label = hierarchyLabel(ref)
			case "brandOptions":
				ref, ok := references.Brands[id]
				if !ok {
					continue
				}
				option.Label = ref.Name
			}
			resolved = append(resolved, option)
		}
		if facet == "availableToOptions" || facet == "specializationOptions" || facet == "brandOptions" {
			result[facet] = resolved
		}
	}
	return result, nil
}

func (s *ServiceCenterReadService) Export(
	ctx context.Context,
	clientID string,
	request dto.ExportServiceCentersRequest,
	access dto.ServiceCenterReadAccess,
) (*coreHelper.ExportResult, error) {
	visibleColumns := make([]string, 0, len(request.Columns))
	exportColumns := make([]coreHelper.DynamicColumn, 0, len(request.Columns))
	for _, column := range request.Columns {
		visibleColumns = append(visibleColumns, column.Key)
		exportColumns = append(exportColumns, coreHelper.DynamicColumn{Key: column.Key, Header: column.Header})
	}
	rows, err := s.repository.ExportRows(ctx, clientID, request, visibleColumns, access)
	if err != nil {
		return nil, err
	}
	references, err := s.resolveReferences(ctx, clientID, rows)
	if err != nil {
		return nil, err
	}
	exportRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		serviceTypes := append([]string(nil), row.ServiceTypes...)
		sort.Strings(serviceTypes)
		status := "Inactive"
		if row.IsActive {
			status = "Active"
		}
		exportRow := map[string]any{
			"name":         row.Name,
			"serviceTypes": strings.Join(serviceTypes, ", "),
			"city":         row.City,
			"isActive":     status,
			"modifiedBy":   modifiedBy(row.ModifiedByName),
		}
		if len(row.GroupIDs) == 0 {
			exportRow["availableTo"] = "-"
		} else {
			exportRow["availableTo"] = strings.Join(hierarchyLabels(row.GroupIDs, references.Groups), ", ")
		}
		if len(row.CategoryIDs) == 0 {
			exportRow["specializations"] = "-"
		} else {
			exportRow["specializations"] = strings.Join(hierarchyLabels(row.CategoryIDs, references.Categories), ", ")
		}
		if len(row.BrandIDs) == 0 {
			exportRow["brands"] = "-"
		} else {
			exportRow["brands"] = strings.Join(brandLabels(row.BrandIDs, references.Brands), ", ")
		}
		exportRows = append(exportRows, exportRow)
	}
	result, err := coreHelper.ExportDynamic(exportRows, exportColumns, coreHelper.ExportParam{
		Format:       request.Format,
		FileName:     "Service-Centers",
		FallbackName: "Service-Centers",
		SheetName:    "Sheet1",
	})
	if err != nil {
		return nil, fmt.Errorf("render service center export: %w", err)
	}
	if request.Format == "csv" && !bytes.HasPrefix(result.Data, []byte{0xEF, 0xBB, 0xBF}) {
		result.Data = append([]byte{0xEF, 0xBB, 0xBF}, result.Data...)
	}
	return result, nil
}

func listItem(row query_builder.ReadCenterRow, roles middleware.UserAccessCache, access dto.ServiceCenterReadAccess, references ReferenceData) dto.ListServiceCenterItem {
	return dto.ListServiceCenterItem{
		ID:                row.ID,
		Name:              row.Name,
		ServiceTypes:      row.ServiceTypes,
		AvailableTo:       resolvedHierarchyRefs(row.GroupIDs, references.Groups),
		Specializations:   resolvedHierarchyRefs(row.CategoryIDs, references.Categories),
		Brands:            resolvedBrandRefs(row.BrandIDs, references.Brands),
		Country:           row.Country,
		City:              row.City,
		ProvinceState:     row.ProvinceState,
		PostalCode:        row.PostalCode,
		AddressLine:       row.AddressLine,
		IsActive:          row.IsActive,
		ModifiedBy:        modifiedBy(row.ModifiedByName),
		UpdatedAt:         row.UpdatedAt,
		SystemPermissions: permissionsFor(row.GroupIDs, roles, access),
	}
}

func emptyHierarchyRefs(ids []int64) *[]dto.HierarchyRef {
	if len(ids) != 0 {
		return nil
	}
	empty := make([]dto.HierarchyRef, 0)
	return &empty
}

func emptyBrandRefs(ids []int64) *[]dto.BrandRef {
	if len(ids) != 0 {
		return nil
	}
	empty := make([]dto.BrandRef, 0)
	return &empty
}

func modifiedBy(name *string) string {
	if name == nil || strings.TrimSpace(*name) == "" {
		return "System"
	}
	return *name
}

func permissionsFor(recordGroupIDs []int64, roles middleware.UserAccessCache, access dto.ServiceCenterReadAccess) dto.ServiceCenterPermissions {
	return dto.ServiceCenterPermissions{
		MaintenanceRoutineRole: permissionForTransaction(recordGroupIDs, roles, access, enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole),
		RepairTicketingRole:    permissionForTransaction(recordGroupIDs, roles, access, enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole),
	}
}

func permissionForTransaction(
	recordGroupIDs []int64,
	roles middleware.UserAccessCache,
	access dto.ServiceCenterReadAccess,
	transaction enum.FixedAssetTransactionRoleTransactionKey,
) dto.ServiceCenterCRUDPermissions {
	if access.TotalControl {
		return dto.ServiceCenterCRUDPermissions{Create: true, Read: true, Update: true, Delete: true}
	}
	if access.ReadOnly {
		return dto.ServiceCenterCRUDPermissions{Read: true}
	}
	manageGroup := enum.SystemRoleKeyManageGroup
	manageAsset := enum.SystemRoleKeyManageAssetAndTAG
	out := dto.ServiceCenterCRUDPermissions{
		Create: len(roles.GroupIDsWithPermission(middleware.PermCreate, manageGroup)) > 0,
		Read:   anyRoleCanRead(recordGroupIDs, roles, transaction),
	}
	if !out.Create {
		out.Create = anyAssetManagerPermission(roles, transaction, middleware.PermCreate)
	}
	out.Update = allRecordGroupsPermitted(recordGroupIDs, roles, manageGroup, middleware.PermEdit) ||
		allRecordGroupsAssetPermitted(recordGroupIDs, roles, transaction, middleware.PermEdit, manageAsset)
	out.Delete = allRecordGroupsPermitted(recordGroupIDs, roles, manageGroup, middleware.PermDelete) ||
		allRecordGroupsAssetPermitted(recordGroupIDs, roles, transaction, middleware.PermDelete, manageAsset)
	return out
}

func anyRoleCanRead(groupIDs []int64, roles middleware.UserAccessCache, transaction enum.FixedAssetTransactionRoleTransactionKey) bool {
	for _, groupID := range groupIDs {
		if roles.HasGroupPermissionFixedAsset(enum.SystemRoleKeyManageGroup, groupID, middleware.PermView) {
			return true
		}
		if roles.HasGroupPermissionFixedAsset(enum.SystemRoleKeyManageAssetAndTAG, groupID, middleware.PermView) &&
			roles.HasTransactionRoleFixedAsset(transaction, groupID, enum.TransactionRolesTypeManager, enum.TransactionRolesTypeMonitoring) {
			return true
		}
	}
	return false
}

func anyAssetManagerPermission(roles middleware.UserAccessCache, transaction enum.FixedAssetTransactionRoleTransactionKey, verb middleware.PermissionVerb) bool {
	for _, groupID := range roles.GroupIDsWithPermission(verb, enum.SystemRoleKeyManageAssetAndTAG) {
		if roles.HasTransactionRoleFixedAsset(transaction, groupID, enum.TransactionRolesTypeManager) {
			return true
		}
	}
	return false
}

func allRecordGroupsPermitted(groupIDs []int64, roles middleware.UserAccessCache, key enum.SystemRoleKey, verb middleware.PermissionVerb) bool {
	if len(groupIDs) == 0 {
		return false
	}
	for _, groupID := range groupIDs {
		if !roles.HasGroupPermissionFixedAsset(key, groupID, verb) {
			return false
		}
	}
	return true
}

func allRecordGroupsAssetPermitted(groupIDs []int64, roles middleware.UserAccessCache, transaction enum.FixedAssetTransactionRoleTransactionKey, verb middleware.PermissionVerb, key enum.SystemRoleKey) bool {
	if len(groupIDs) == 0 {
		return false
	}
	for _, groupID := range groupIDs {
		if !roles.HasGroupPermissionFixedAsset(key, groupID, verb) ||
			!roles.HasTransactionRoleFixedAsset(transaction, groupID, enum.TransactionRolesTypeManager) {
			return false
		}
	}
	return true
}

type ReferenceData struct {
	Groups     map[int64]dto.HierarchyRef
	Categories map[int64]dto.HierarchyRef
	Brands     map[int64]dto.BrandRef
}

type ServiceCenterReferenceResolver interface {
	Resolve(context.Context, string, []int64, []int64, []int64) (ReferenceData, error)
}

func (s *ServiceCenterReadService) resolveReferences(ctx context.Context, clientID string, rows []query_builder.ReadCenterRow) (ReferenceData, error) {
	groupIDs := make([]int64, 0)
	categoryIDs := make([]int64, 0)
	brandIDs := make([]int64, 0)
	groupSet := map[int64]struct{}{}
	categorySet := map[int64]struct{}{}
	brandSet := map[int64]struct{}{}
	for _, row := range rows {
		for _, id := range row.GroupIDs {
			groupSet[id] = struct{}{}
		}
		for _, id := range row.CategoryIDs {
			categorySet[id] = struct{}{}
		}
		for _, id := range row.BrandIDs {
			brandSet[id] = struct{}{}
		}
	}
	for id := range groupSet {
		groupIDs = append(groupIDs, id)
	}
	for id := range categorySet {
		categoryIDs = append(categoryIDs, id)
	}
	for id := range brandSet {
		brandIDs = append(brandIDs, id)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	sort.Slice(categoryIDs, func(i, j int) bool { return categoryIDs[i] < categoryIDs[j] })
	sort.Slice(brandIDs, func(i, j int) bool { return brandIDs[i] < brandIDs[j] })
	data := ReferenceData{
		Groups:     make(map[int64]dto.HierarchyRef),
		Categories: make(map[int64]dto.HierarchyRef),
		Brands:     make(map[int64]dto.BrandRef),
	}
	if s.references == nil || (len(groupIDs) == 0 && len(categoryIDs) == 0 && len(brandIDs) == 0) {
		return data, nil
	}
	return s.references.Resolve(ctx, clientID, groupIDs, categoryIDs, brandIDs)
}

func optionIDs(options []dto.OptionItem) []int64 {
	seen := make(map[int64]struct{}, len(options))
	ids := make([]int64, 0, len(options))
	for _, option := range options {
		id, err := strconv.ParseInt(option.Value, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func resolvedHierarchyRefs(ids []int64, resolved map[int64]dto.HierarchyRef) *[]dto.HierarchyRef {
	refs := make([]dto.HierarchyRef, 0, len(ids))
	for _, id := range ids {
		ref, ok := resolved[id]
		if !ok {
			ref = dto.HierarchyRef{ID: id}
		}
		if ref.ID == 0 {
			ref.ID = id
		}
		refs = append(refs, ref)
	}
	return &refs
}

func resolvedBrandRefs(ids []int64, resolved map[int64]dto.BrandRef) *[]dto.BrandRef {
	refs := make([]dto.BrandRef, 0, len(ids))
	for _, id := range ids {
		ref, ok := resolved[id]
		if !ok {
			ref = dto.BrandRef{ID: id}
		}
		if ref.ID == 0 {
			ref.ID = id
		}
		refs = append(refs, ref)
	}
	return &refs
}

func hierarchyLabel(ref dto.HierarchyRef) string {
	if ref.Path != "" {
		return ref.Path
	}
	return ref.Name
}

func hierarchyLabels(ids []int64, resolved map[int64]dto.HierarchyRef) []string {
	labels := make([]string, 0, len(ids))
	for _, ref := range *resolvedHierarchyRefs(ids, resolved) {
		label := hierarchyLabel(ref)
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func brandLabels(ids []int64, resolved map[int64]dto.BrandRef) []string {
	labels := make([]string, 0, len(ids))
	for _, ref := range *resolvedBrandRefs(ids, resolved) {
		if ref.Name != "" {
			labels = append(labels, ref.Name)
		}
	}
	return labels
}

type HTTPServiceCenterReferenceResolver struct {
	masterAssetRegistryURL  string
	enterpriseManagementURL string
	serviceToken            string
	client                  *http.Client
}

func NewHTTPServiceCenterReferenceResolver(masterAssetRegistryURL, enterpriseManagementURL, serviceToken string) *HTTPServiceCenterReferenceResolver {
	return &HTTPServiceCenterReferenceResolver{
		masterAssetRegistryURL:  strings.TrimRight(masterAssetRegistryURL, "/"),
		enterpriseManagementURL: strings.TrimRight(enterpriseManagementURL, "/"),
		serviceToken:            serviceToken,
		client:                  &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *HTTPServiceCenterReferenceResolver) Resolve(ctx context.Context, clientID string, groupIDs, categoryIDs, brandIDs []int64) (ReferenceData, error) {
	data := ReferenceData{
		Groups:     make(map[int64]dto.HierarchyRef),
		Categories: make(map[int64]dto.HierarchyRef),
		Brands:     make(map[int64]dto.BrandRef),
	}
	if len(groupIDs) > 0 {
		var groups []struct {
			ID       json.RawMessage `json:"_id"`
			Name     string          `json:"name"`
			FullPath string          `json:"fullPath"`
		}
		if err := r.getData(ctx, r.enterpriseManagementURL, "/internal/groups", clientID, &groups); err != nil {
			return ReferenceData{}, err
		}
		for _, group := range groups {
			id, ok := parseReferenceID(group.ID)
			if !ok {
				continue
			}
			data.Groups[id] = dto.HierarchyRef{ID: id, Name: group.Name, Path: group.FullPath}
		}
	}
	if len(categoryIDs) > 0 {
		for start := 0; start < len(categoryIDs); start += 200 {
			end := start + 200
			if end > len(categoryIDs) {
				end = len(categoryIDs)
			}
			request := struct {
				CategoryIDs []string `json:"categoryIds"`
			}{CategoryIDs: make([]string, 0, end-start)}
			for _, id := range categoryIDs[start:end] {
				request.CategoryIDs = append(request.CategoryIDs, strconv.FormatInt(id, 10))
			}
			var response struct {
				Results []struct {
					ID       json.RawMessage `json:"id"`
					Found    bool            `json:"found"`
					Name     *string         `json:"name"`
					FullPath *string         `json:"fullPath"`
				} `json:"results"`
			}
			if err := r.postData(ctx, r.masterAssetRegistryURL, "/internal/categories/lookup", clientID, request, &response); err != nil {
				return ReferenceData{}, err
			}
			for _, item := range response.Results {
				id, ok := parseReferenceID(item.ID)
				if !ok || !item.Found {
					continue
				}
				ref := dto.HierarchyRef{ID: id}
				if item.Name != nil {
					ref.Name = *item.Name
				}
				if item.FullPath != nil {
					ref.Path = *item.FullPath
				}
				data.Categories[id] = ref
			}
		}
	}
	if len(brandIDs) > 0 {
		var brands []struct {
			ID   json.RawMessage `json:"_id"`
			Name string          `json:"name"`
		}
		if err := r.getData(ctx, r.enterpriseManagementURL, "/internal/brands/dropdown", clientID, &brands); err != nil {
			return ReferenceData{}, err
		}
		for _, brand := range brands {
			id, ok := parseReferenceID(brand.ID)
			if !ok {
				continue
			}
			data.Brands[id] = dto.BrandRef{ID: id, Name: brand.Name}
		}
	}
	return data, nil
}

func (r *HTTPServiceCenterReferenceResolver) getData(ctx context.Context, baseURL, path, clientID string, out any) error {
	raw, err := r.call(ctx, http.MethodGet, baseURL, path, clientID, nil)
	if err != nil {
		return err
	}
	return decodeReferenceData(raw, out)
}

func (r *HTTPServiceCenterReferenceResolver) postData(ctx context.Context, baseURL, path, clientID string, payload, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode service center reference request %s: %w", path, err)
	}
	response, err := r.call(ctx, http.MethodPost, baseURL, path, clientID, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return decodeReferenceData(response, out)
}

func (r *HTTPServiceCenterReferenceResolver) call(ctx context.Context, method, baseURL, path, clientID string, body io.Reader) ([]byte, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("service center reference endpoint %s is not configured", path)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build service center reference request %s: %w", path, err)
	}
	request.Header.Set("X-Service-Token", r.serviceToken)
	request.Header.Set("X-Client-ID", clientID)
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := r.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call service center reference endpoint %s: %w", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read service center reference endpoint %s: %w", path, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("service center reference endpoint %s returned status %d", path, response.StatusCode)
	}
	return raw, nil
}

func decodeReferenceData(raw []byte, out any) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode service center reference envelope: %w", err)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("service center reference response has no data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode service center reference data: %w", err)
	}
	return nil
}

func parseReferenceID(raw json.RawMessage) (int64, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return 0, false
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, false
		}
		text = strings.TrimSpace(value)
	}
	id, err := strconv.ParseInt(text, 10, 64)
	return id, err == nil && id > 0
}

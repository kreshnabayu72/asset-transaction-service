package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ServiceCenterListColumns = map[string]struct{}{
	"name": {}, "serviceTypes": {}, "specializations": {}, "availableTo": {},
	"city": {}, "brands": {}, "usage": {}, "isActive": {}, "modifiedBy": {}, "updatedAt": {},
}

type ReadFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ServiceCenterReadFilters struct {
	SortBy         string   `json:"sortBy"`
	SortOrder      int      `json:"sortOrder"`
	Search         string   `json:"search"`
	ServiceType    []string `json:"serviceType"`
	Specialization []string `json:"specialization"`
	AvailableTo    []string `json:"availableTo"`
	Brand          []string `json:"brand"`
	Status         []string `json:"status"`
	Country        []string `json:"country"`
	City           []string `json:"city"`
	ModifiedBy     []string `json:"modifiedBy"`
	UpdatedAt      []int64  `json:"updatedAt"`
	UsageMin       *int     `json:"usageMin"`
	UsageMax       *int     `json:"usageMax"`
}

type ServiceCenterReadAccess struct {
	GroupIDs     []int64
	TotalControl bool
	ReadOnly     bool
}

type ListServiceCentersRequest struct {
	ServiceCenterReadFilters
	Page    int      `json:"page"`
	Limit   int      `json:"limit"`
	Columns []string `json:"columns"`
}

type ExportColumn struct {
	Key    string `json:"key"`
	Header string `json:"header"`
}

type ExportServiceCentersRequest struct {
	ServiceCenterReadFilters
	Columns []ExportColumn `json:"columns"`
	Format  string         `json:"format"`
	Page    *int           `json:"page,omitempty"`
	Limit   *int           `json:"limit,omitempty"`
}

type OptionsServiceCentersRequest map[string]json.RawMessage

type OptionItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type HierarchyRef struct {
	ID   int64  `json:"_id,string"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type BrandRef struct {
	ID       int64  `json:"_id,string"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

type ServiceCenterCRUDPermissions struct {
	Create bool `json:"create"`
	Read   bool `json:"read"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

type ServiceCenterPermissions struct {
	MaintenanceRoutineRole ServiceCenterCRUDPermissions `json:"maintenanceRoutineRole"`
	RepairTicketingRole    ServiceCenterCRUDPermissions `json:"repairTicketingRole"`
}

type ListServiceCenterItem struct {
	ID                int64                    `json:"_id,string"`
	Name              string                   `json:"name"`
	ServiceTypes      []string                 `json:"serviceTypes"`
	Specializations   *[]HierarchyRef          `json:"specializations,omitempty"`
	AvailableTo       *[]HierarchyRef          `json:"availableTo,omitempty"`
	Country           string                   `json:"country"`
	City              string                   `json:"city"`
	ProvinceState     *string                  `json:"provinceState,omitempty"`
	PostalCode        *string                  `json:"postalCode,omitempty"`
	AddressLine       *string                  `json:"addressLine,omitempty"`
	Brands            *[]BrandRef              `json:"brands,omitempty"`
	Usage             *int                     `json:"usage,omitempty"`
	IsActive          bool                     `json:"isActive"`
	ModifiedBy        string                   `json:"modifiedBy"`
	UpdatedAt         time.Time                `json:"updatedAt"`
	SystemPermissions ServiceCenterPermissions `json:"systemPermissions"`
	IsInUse           *bool                    `json:"isInUse,omitempty"`
}

type ServiceCenterListResponse struct {
	Message string `json:"message"`
	Data    struct {
		Data         []ListServiceCenterItem `json:"data"`
		TotalRecords int                     `json:"totalRecords"`
	} `json:"data"`
}

type Phone struct {
	DialCode string `json:"dialCode"`
	Number   string `json:"number"`
}

type ContactPerson struct {
	Name      string  `json:"name"`
	Phone     *Phone  `json:"phone"`
	Email     *string `json:"email"`
	IsPrimary bool    `json:"isPrimary"`
}

type NamedRef struct {
	ID   int64  `json:"_id,string"`
	Name string `json:"name"`
}

type ServiceCenterAttachment struct {
	FileID   int64  `json:"fileId,string"`
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
	FileType string `json:"fileType"`
	URL      string `json:"url,omitempty"`
}

type ServiceCenterDetail struct {
	ID                       int64                    `json:"_id,string"`
	Name                     string                   `json:"name"`
	IsActive                 bool                     `json:"isActive"`
	ServiceTypes             []string                 `json:"serviceTypes"`
	AvailableTo              *[]HierarchyRef          `json:"availableTo,omitempty"`
	Specializations          *[]HierarchyRef          `json:"specializations,omitempty"`
	Brands                   *[]BrandRef              `json:"brands,omitempty"`
	Country                  string                   `json:"country"`
	ProvinceState            *string                  `json:"provinceState"`
	City                     string                   `json:"city"`
	PostalCode               *string                  `json:"postalCode"`
	AddressLine              *string                  `json:"addressLine"`
	CompanyPhone             *Phone                   `json:"companyPhone"`
	CompanyEmail             *string                  `json:"companyEmail"`
	ContactPersons           []ContactPerson          `json:"contactPersons"`
	Notes                    *string                  `json:"notes"`
	Attachment               *ServiceCenterAttachment `json:"attachment,omitempty"`
	Usage                    *int                     `json:"usage,omitempty"`
	CreatedBy                NamedRef                 `json:"createdBy"`
	CreatedAt                time.Time                `json:"createdAt"`
	ModifiedBy               string                   `json:"modifiedBy"`
	UpdatedAt                time.Time                `json:"updatedAt"`
	Version                  int32                    `json:"version"`
	ServesGroupsOutsideScope *bool                    `json:"servesGroupsOutsideScope,omitempty"`
	SystemPermissions        ServiceCenterPermissions `json:"systemPermissions"`
	IsInUse                  *bool                    `json:"isInUse,omitempty"`
}

type ServiceCenterDetailResponse struct {
	Message string              `json:"message"`
	Data    ServiceCenterDetail `json:"data"`
}

func (r ServiceCenterReadFilters) ValidateFilters() []ReadFieldError {
	var errors []ReadFieldError
	if _, ok := ServiceCenterListColumns[r.SortBy]; !ok {
		errors = append(errors, ReadFieldError{"sortBy", "sortBy is not supported"})
	}
	if r.SortOrder != 1 && r.SortOrder != -1 {
		errors = append(errors, ReadFieldError{"sortOrder", "sortOrder must be 1 or -1"})
	}
	for _, value := range r.ServiceType {
		if value != "Routine" && value != "Repair" {
			errors = append(errors, ReadFieldError{"serviceType", "serviceType must contain Routine or Repair"})
			break
		}
	}
	for _, value := range r.Status {
		if value != "Active" && value != "Inactive" {
			errors = append(errors, ReadFieldError{"status", "status must contain Active or Inactive"})
			break
		}
	}
	for _, field := range []struct {
		name string
		ids  []string
	}{{"specialization", r.Specialization}, {"availableTo", r.AvailableTo}, {"brand", r.Brand}} {
		for _, value := range field.ids {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				errors = append(errors, ReadFieldError{field.name, field.name + " values must be positive decimal IDs"})
				break
			}
		}
	}
	if len(r.UpdatedAt) != 0 && len(r.UpdatedAt) != 2 {
		errors = append(errors, ReadFieldError{"updatedAt", "updatedAt must contain exactly two epoch millisecond values"})
	} else if len(r.UpdatedAt) == 2 && r.UpdatedAt[0] > r.UpdatedAt[1] {
		errors = append(errors, ReadFieldError{"updatedAt", "updatedAt range must be ordered"})
	}
	if r.UsageMin != nil && *r.UsageMin < 0 {
		errors = append(errors, ReadFieldError{"usageMin", "usageMin must be non-negative"})
	}
	if r.UsageMax != nil && *r.UsageMax < 0 {
		errors = append(errors, ReadFieldError{"usageMax", "usageMax must be non-negative"})
	}
	if r.UsageMin != nil && r.UsageMax != nil && *r.UsageMin > *r.UsageMax {
		errors = append(errors, ReadFieldError{"usageMax", "usageMax must be greater than or equal to usageMin"})
	}
	return errors
}

func (r ListServiceCentersRequest) Validate() []ReadFieldError {
	errors := r.ServiceCenterReadFilters.ValidateFilters()
	if r.Page < 1 {
		errors = append(errors, ReadFieldError{"page", "page must be at least 1"})
	}
	if r.Limit < 1 || r.Limit > 100 {
		errors = append(errors, ReadFieldError{"limit", "limit must be between 1 and 100"})
	}
	if r.Columns == nil {
		errors = append(errors, ReadFieldError{"columns", "columns is required"})
	}
	return errors
}

func (r ExportServiceCentersRequest) Validate() []ReadFieldError {
	errors := r.ServiceCenterReadFilters.ValidateFilters()
	if r.Format != "csv" && r.Format != "xlsx" {
		errors = append(errors, ReadFieldError{"format", "format must be csv or xlsx"})
	}
	if r.Page != nil || r.Limit != nil {
		errors = append(errors, ReadFieldError{"page", "page and limit are not supported for export"})
	}
	if len(r.Columns) == 0 {
		errors = append(errors, ReadFieldError{"columns", "at least one export column is required"})
	}
	seen := make(map[string]struct{})
	for i, column := range r.Columns {
		if _, ok := ServiceCenterListColumns[column.Key]; !ok {
			errors = append(errors, ReadFieldError{fmt.Sprintf("columns[%d].key", i), "unknown export column"})
		}
		if _, ok := seen[column.Key]; ok {
			errors = append(errors, ReadFieldError{fmt.Sprintf("columns[%d].key", i), "duplicate export column"})
		}
		seen[column.Key] = struct{}{}
		n := len([]rune(column.Header))
		if n < 1 || n > 100 {
			errors = append(errors, ReadFieldError{fmt.Sprintf("columns[%d].header", i), "header must contain 1 to 100 characters"})
		}
	}
	return errors
}

func (r ListServiceCentersRequest) VisibleColumns() []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(r.Columns))
	for _, key := range r.Columns {
		if _, valid := ServiceCenterListColumns[key]; !valid {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func (r ServiceCenterReadFilters) PositiveIDs(values []string) ([]int64, error) {
	out := make([]int64, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid positive ID %q", value)
		}
		out = append(out, id)
	}
	return out, nil
}

func (r ServiceCenterReadFilters) SearchTerm() string { return strings.TrimSpace(r.Search) }

func (r OptionsServiceCentersRequest) RequestedFacets() (map[string]bool, []ReadFieldError) {
	allowed := map[string]struct{}{"serviceTypeOptions": {}, "specializationOptions": {}, "availableToOptions": {}, "brandOptions": {}, "statusOptions": {}, "countryOptions": {}, "cityOptions": {}, "modifiedByOptions": {}}
	result := make(map[string]bool)
	var errors []ReadFieldError
	for key, raw := range r {
		if _, ok := allowed[key]; !ok {
			errors = append(errors, ReadFieldError{key, "unknown facet"})
			continue
		}
		var enabled bool
		if err := json.Unmarshal(raw, &enabled); err != nil {
			errors = append(errors, ReadFieldError{key, key + " must be a boolean"})
			continue
		}
		if enabled {
			result[key] = true
		}
	}
	return result, errors
}

func UnknownReadRequestFields(body []byte, allowedKeys ...string) []ReadFieldError {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, key := range allowedKeys {
		allowed[key] = struct{}{}
	}
	var errors []ReadFieldError
	for key := range decoded {
		if _, ok := allowed[key]; !ok {
			errors = append(errors, ReadFieldError{Field: key, Message: "unknown field"})
		}
	}
	return errors
}

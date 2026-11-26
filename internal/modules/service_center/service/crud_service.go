package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/jackc/pgx/v5/pgconn"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	crudTypes "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/crud"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

var (
	ErrCRUDNotFound                     = errors.New("service center not found")
	ErrCRUDDuplicateName                = errors.New("service center name already exists")
	ErrCRUDUsageProviderUnavailable     = errors.New("service center usage provider is unavailable")
	ErrCRUDAttachmentContractUnresolved = errors.New("service center attachment lifecycle contract is unresolved")
	ErrCRUDScopeProviderUnavailable     = errors.New("service center hierarchy provider is unavailable")
	ErrCRUDIDGeneratorUnavailable       = errors.New("Service Center Snowflake ID generator is unavailable")
)

type CRUDIDGenerator interface {
	NextID() int64
}

type SnowflakeCRUDIDGenerator struct {
	node *snowflake.Node
}

func NewSnowflakeCRUDIDGenerator(nodeID int64) (*SnowflakeCRUDIDGenerator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("create Service Center Snowflake node: %w", err)
	}
	return &SnowflakeCRUDIDGenerator{node: node}, nil
}

func (g *SnowflakeCRUDIDGenerator) NextID() int64 {
	return g.node.Generate().Int64()
}

type CRUDWriteData = crudTypes.WriteData

type CRUDTransaction = crudTypes.Transaction

type CRUDRepository = crudTypes.Repository

type CRUDService struct {
	repository CRUDRepository
	ids        CRUDIDGenerator
}

func NewCRUDService(repository CRUDRepository, ids CRUDIDGenerator) *CRUDService {
	return &CRUDService{repository: repository, ids: ids}
}

type CRUDVersionConflictError struct {
	Current model.ServiceCenter
}

func (e *CRUDVersionConflictError) Error() string {
	return "service center version changed"
}

func (s *CRUDService) CreateServiceCenter(
	ctx context.Context,
	clientID string,
	request dto.CreateServiceCenterRequest,
	actorID int64,
	actorName string,
) error {
	request.Normalize()
	if s.ids == nil {
		return ErrCRUDIDGeneratorUnavailable
	}
	if request.AttachmentFileID.Value != nil {
		return ErrCRUDAttachmentContractUnresolved
	}
	data, err := s.makeWriteData(request.ServiceCenterWriteRequest, actorID, actorName)
	if err != nil {
		return err
	}

	tx, err := s.repository.BeginCRUDTransaction(ctx, clientID)
	if err != nil {
		return fmt.Errorf("begin Service Center create transaction: %w", err)
	}
	defer tx.Close()
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	available, err := tx.NameAvailable(ctx, data.Center.Name, nil)
	if err != nil {
		return fmt.Errorf("check Service Center name availability: %w", err)
	}
	if !available {
		return ErrCRUDDuplicateName
	}

	data.Center.ID = s.ids.NextID()
	data.Center.NameKey = strings.ToLower(data.Center.Name)
	data.Center.IsActive = true
	data.Center.Version = 1
	data.Center.CreatedByID = actorID
	data.Center.CreatedByName = actorName
	data.Center.CreatedAt = time.Now().UTC()
	data.Center.UpdatedAt = data.Center.CreatedAt
	modifiedByID := actorID
	modifiedByName := actorName
	data.Center.ModifiedByID = &modifiedByID
	data.Center.ModifiedByName = &modifiedByName

	if err := tx.InsertServiceCenter(ctx, data.Center); err != nil {
		if isNameKeyUniqueViolation(err) {
			return ErrCRUDDuplicateName
		}
		return fmt.Errorf("insert Service Center: %w", err)
	}
	if err := insertCRUDChildren(ctx, tx, data); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Service Center create transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *CRUDService) UpdateServiceCenter(
	ctx context.Context,
	clientID string,
	id int64,
	request dto.UpdateServiceCenterRequest,
	actorID int64,
	actorName string,
) (bool, error) {
	request.Normalize()
	if s.ids == nil && len(request.ContactPersons) > 0 {
		return false, ErrCRUDIDGeneratorUnavailable
	}
	data, err := s.makeWriteData(request.ServiceCenterWriteRequest, actorID, actorName)
	if err != nil {
		return false, err
	}

	tx, err := s.repository.BeginCRUDTransaction(ctx, clientID)
	if err != nil {
		return false, fmt.Errorf("begin Service Center update transaction: %w", err)
	}
	defer tx.Close()
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	current, found, err := tx.LockServiceCenter(ctx, id)
	if err != nil {
		return false, fmt.Errorf("lock Service Center for update: %w", err)
	}
	if !found {
		return false, ErrCRUDNotFound
	}
	if request.AttachmentFileID.Set {
		if request.AttachmentFileID.Value != nil {
			return false, ErrCRUDAttachmentContractUnresolved
		}
		hasAttachment, err := tx.HasAttachment(ctx, id)
		if err != nil {
			return false, fmt.Errorf("check Service Center attachment: %w", err)
		}
		if hasAttachment {
			return false, ErrCRUDAttachmentContractUnresolved
		}
	}
	currentData, err := loadCurrentCRUDData(ctx, tx, current)
	if err != nil {
		return false, err
	}
	data.Center.ID = id
	data.Center.NameKey = strings.ToLower(data.Center.Name)
	if sameCRUDWrite(currentData, data) {
		return false, nil
	}
	if request.Version != current.Version {
		return false, &CRUDVersionConflictError{Current: current}
	}
	if removesServiceType(current.ServiceTypes, data.Center.ServiceTypes) {
		return false, ErrCRUDUsageProviderUnavailable
	}
	if !sameIDSet(currentData.GroupIDs, data.GroupIDs) ||
		!sameIDSet(currentData.CategoryIDs, data.CategoryIDs) ||
		!sameIDSet(currentData.BrandIDs, data.BrandIDs) {
		return false, ErrCRUDScopeProviderUnavailable
	}

	available, err := tx.NameAvailable(ctx, data.Center.Name, &id)
	if err != nil {
		return false, fmt.Errorf("check Service Center name availability: %w", err)
	}
	if !available {
		return false, ErrCRUDDuplicateName
	}
	if err := tx.UpdateServiceCenter(ctx, id, request.Version, data); err != nil {
		if isNameKeyUniqueViolation(err) {
			return false, ErrCRUDDuplicateName
		}
		return false, fmt.Errorf("update Service Center fields: %w", err)
	}
	for _, table := range []string{"service_center_groups", "service_center_categories", "service_center_brands", "service_center_contacts"} {
		if err := tx.DeleteChildRows(ctx, table, id); err != nil {
			return false, fmt.Errorf("replace Service Center child rows: %w", err)
		}
	}
	if err := insertCRUDChildren(ctx, tx, data); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit Service Center update transaction: %w", err)
	}
	committed = true
	return true, nil
}

func (s *CRUDService) DeleteServiceCenter(ctx context.Context, clientID string, id int64, version int32) error {
	tx, err := s.repository.BeginCRUDTransaction(ctx, clientID)
	if err != nil {
		return fmt.Errorf("begin Service Center delete transaction: %w", err)
	}
	defer tx.Close()
	defer func() { _ = tx.Rollback(ctx) }()

	current, found, err := tx.LockServiceCenter(ctx, id)
	if err != nil {
		return fmt.Errorf("lock Service Center for delete: %w", err)
	}
	if !found {
		return ErrCRUDNotFound
	}
	if current.Version != version {
		return &CRUDVersionConflictError{Current: current}
	}
	hasAttachment, err := tx.HasAttachment(ctx, id)
	if err != nil {
		return fmt.Errorf("check Service Center attachment: %w", err)
	}
	if hasAttachment {
		return ErrCRUDAttachmentContractUnresolved
	}
	// Deletion remains fail-closed until the authoritative maintenance-routine usage
	// provider is available; deleting without its reference count could orphan work.
	return ErrCRUDUsageProviderUnavailable
}

func (s *CRUDService) makeWriteData(
	request dto.ServiceCenterWriteRequest,
	actorID int64,
	actorName string,
) (CRUDWriteData, error) {
	if s.ids == nil && len(request.ContactPersons) > 0 {
		return CRUDWriteData{}, ErrCRUDIDGeneratorUnavailable
	}
	groupIDs, err := parseCRUDIDs(request.AvailableTo)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("parse availableTo IDs: %w", err)
	}
	categoryIDs, err := parseCRUDIDs(request.Specializations)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("parse specialization IDs: %w", err)
	}
	brandIDs, err := parseCRUDIDs(request.Brands)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("parse brand IDs: %w", err)
	}
	modifiedByID := actorID
	modifiedByName := actorName
	center := model.ServiceCenter{
		Name:                 request.Name,
		ServiceTypes:         append([]string(nil), request.ServiceTypes...),
		Country:              request.Country,
		ProvinceState:        request.ProvinceState,
		City:                 request.City,
		PostalCode:           request.PostalCode,
		AddressLine:          request.AddressLine,
		CompanyEmail:         request.CompanyEmail,
		Notes:                request.Notes,
		CreatedByID:          actorID,
		CreatedByName:        actorName,
		ModifiedByID:         &modifiedByID,
		ModifiedByName:       &modifiedByName,
		CompanyPhoneDialCode: phoneDialCode(request.CompanyPhone),
		CompanyPhoneNumber:   phoneNumber(request.CompanyPhone),
	}
	data := CRUDWriteData{
		Center:      center,
		GroupIDs:    groupIDs,
		CategoryIDs: categoryIDs,
		BrandIDs:    brandIDs,
		Contacts:    make([]model.ServiceCenterContact, 0, len(request.ContactPersons)),
	}
	for i, person := range request.ContactPersons {
		data.Contacts = append(data.Contacts, model.ServiceCenterContact{
			ID:            s.ids.NextID(),
			Position:      int32(i),
			Name:          person.Name,
			PhoneDialCode: phoneDialCode(person.Phone),
			PhoneNumber:   phoneNumber(person.Phone),
			Email:         person.Email,
			IsPrimary:     person.IsPrimary,
		})
	}
	return data, nil
}

func insertCRUDChildren(ctx context.Context, tx CRUDTransaction, data CRUDWriteData) error {
	centerID := data.Center.ID
	for _, id := range data.GroupIDs {
		if err := tx.InsertGroup(ctx, model.ServiceCenterGroup{ServiceCenterID: centerID, GroupID: id}); err != nil {
			return fmt.Errorf("insert Service Center group: %w", err)
		}
	}
	for _, id := range data.CategoryIDs {
		if err := tx.InsertCategory(ctx, model.ServiceCenterCategory{ServiceCenterID: centerID, CategoryID: id}); err != nil {
			return fmt.Errorf("insert Service Center category: %w", err)
		}
	}
	for _, id := range data.BrandIDs {
		if err := tx.InsertBrand(ctx, model.ServiceCenterBrand{ServiceCenterID: centerID, BrandID: id}); err != nil {
			return fmt.Errorf("insert Service Center brand: %w", err)
		}
	}
	for _, contact := range data.Contacts {
		contact.ServiceCenterID = centerID
		if err := tx.InsertContact(ctx, contact); err != nil {
			return fmt.Errorf("insert Service Center contact: %w", err)
		}
	}
	return nil
}

func loadCurrentCRUDData(ctx context.Context, tx CRUDTransaction, center model.ServiceCenter) (CRUDWriteData, error) {
	groups, err := tx.LoadGroupIDs(ctx, center.ID)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("load Service Center groups: %w", err)
	}
	categories, err := tx.LoadCategoryIDs(ctx, center.ID)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("load Service Center categories: %w", err)
	}
	brands, err := tx.LoadBrandIDs(ctx, center.ID)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("load Service Center brands: %w", err)
	}
	contacts, err := tx.LoadContacts(ctx, center.ID)
	if err != nil {
		return CRUDWriteData{}, fmt.Errorf("load Service Center contacts: %w", err)
	}
	return CRUDWriteData{Center: center, GroupIDs: groups, CategoryIDs: categories, BrandIDs: brands, Contacts: contacts}, nil
}

func sameCRUDWrite(current, submitted CRUDWriteData) bool {
	a, b := current.Center, submitted.Center
	if !strings.EqualFold(a.Name, b.Name) ||
		!equalStringSlices(a.ServiceTypes, b.ServiceTypes) ||
		!strings.EqualFold(a.Country, b.Country) ||
		!equalOptionalString(a.ProvinceState, b.ProvinceState) ||
		!strings.EqualFold(a.City, b.City) ||
		!equalOptionalString(a.PostalCode, b.PostalCode) ||
		!equalOptionalString(a.AddressLine, b.AddressLine) ||
		!equalOptionalString(a.CompanyPhoneDialCode, b.CompanyPhoneDialCode) ||
		!equalOptionalString(a.CompanyPhoneNumber, b.CompanyPhoneNumber) ||
		!equalOptionalString(a.CompanyEmail, b.CompanyEmail) ||
		!equalOptionalString(a.Notes, b.Notes) {
		return false
	}
	return sameIDSet(current.GroupIDs, submitted.GroupIDs) &&
		sameIDSet(current.CategoryIDs, submitted.CategoryIDs) &&
		sameIDSet(current.BrandIDs, submitted.BrandIDs) &&
		equalContacts(current.Contacts, submitted.Contacts)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return strings.EqualFold(*a, *b)
}

func sameIDSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	aa, bb := append([]int64(nil), a...), append([]int64(nil), b...)
	sort.Slice(aa, func(i, j int) bool { return aa[i] < aa[j] })
	sort.Slice(bb, func(i, j int) bool { return bb[i] < bb[j] })
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func equalContacts(a, b []model.ServiceCenterContact) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Position != int32(i) || b[i].Position != int32(i) ||
			!strings.EqualFold(a[i].Name, b[i].Name) ||
			!equalOptionalString(a[i].PhoneDialCode, b[i].PhoneDialCode) ||
			!equalOptionalString(a[i].PhoneNumber, b[i].PhoneNumber) ||
			!equalOptionalString(a[i].Email, b[i].Email) ||
			a[i].IsPrimary != b[i].IsPrimary {
			return false
		}
	}
	return true
}

func removesServiceType(current, submitted []string) bool {
	for _, oldType := range current {
		found := false
		for _, newType := range submitted {
			if oldType == newType {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}

func parseCRUDIDs(values []string) ([]int64, error) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid positive decimal ID")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func phoneDialCode(phone *dto.ServiceCenterPhone) *string {
	if phone == nil || phone.Number == "" {
		return nil
	}
	value := phone.DialCode
	return &value
}

func phoneNumber(phone *dto.ServiceCenterPhone) *string {
	if phone == nil || phone.Number == "" {
		return nil
	}
	value := phone.Number
	return &value
}

func isNameKeyUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "service_centers_name_key_key"
}

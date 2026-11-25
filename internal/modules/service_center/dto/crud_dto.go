package dto

import (
	"encoding/json"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ServiceCenterPhone struct {
	DialCode string `json:"dialCode"`
	Number   string `json:"number"`
}

type ServiceCenterContactPerson struct {
	Name      string              `json:"name"`
	Phone     *ServiceCenterPhone `json:"phone"`
	Email     *string             `json:"email"`
	IsPrimary bool                `json:"isPrimary"`
}

// ServiceCenterWriteRequest is the shared field set for create and replace-style edit.
type OptionalString struct {
	Value *string
	Set   bool
}

func (value *OptionalString) UnmarshalJSON(data []byte) error {
	value.Set = true
	if string(data) == "null" {
		value.Value = nil
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	value.Value = &text
	return nil
}

type ServiceCenterWriteRequest struct {
	Name             string                       `json:"name"`
	ServiceTypes     []string                     `json:"serviceTypes"`
	AvailableTo      []string                     `json:"availableTo"`
	Specializations  []string                     `json:"specializations"`
	Brands           []string                     `json:"brands"`
	Country          string                       `json:"country"`
	ProvinceState    *string                      `json:"provinceState"`
	City             string                       `json:"city"`
	PostalCode       *string                      `json:"postalCode"`
	AddressLine      *string                      `json:"addressLine"`
	CompanyPhone     *ServiceCenterPhone          `json:"companyPhone"`
	CompanyEmail     *string                      `json:"companyEmail"`
	ContactPersons   []ServiceCenterContactPerson `json:"contactPersons"`
	Notes            *string                      `json:"notes"`
	AttachmentFileID OptionalString               `json:"attachmentFileId"`
}

type CreateServiceCenterRequest struct {
	ServiceCenterWriteRequest
}

type UpdateServiceCenterRequest struct {
	ServiceCenterWriteRequest
	Version        int32 `json:"version"`
	versionPresent bool
}

func (r *UpdateServiceCenterRequest) UnmarshalJSON(data []byte) error {
	type requestAlias UpdateServiceCenterRequest
	var decoded struct {
		*requestAlias
		Version json.RawMessage `json:"version"`
	}
	decoded.requestAlias = (*requestAlias)(r)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if len(decoded.Version) == 0 || string(decoded.Version) == "null" {
		r.Version = 0
		r.versionPresent = false
		return nil
	}
	if err := json.Unmarshal(decoded.Version, &r.Version); err != nil {
		return err
	}
	r.versionPresent = true
	return nil
}

type CRUDFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (r *ServiceCenterWriteRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Country = strings.TrimSpace(r.Country)
	r.City = strings.TrimSpace(r.City)
	r.ProvinceState = trimOptional(r.ProvinceState)
	r.PostalCode = trimOptional(r.PostalCode)
	r.AddressLine = trimOptional(r.AddressLine)
	r.Notes = trimOptional(r.Notes)
	r.CompanyEmail = trimOptional(r.CompanyEmail)
	if r.CompanyPhone != nil {
		r.CompanyPhone.DialCode = strings.TrimSpace(r.CompanyPhone.DialCode)
		r.CompanyPhone.Number = strings.TrimSpace(r.CompanyPhone.Number)
		if r.CompanyPhone.DialCode == "" && r.CompanyPhone.Number == "" {
			r.CompanyPhone = nil
		}
	}
	for i := range r.ContactPersons {
		r.ContactPersons[i].Name = strings.TrimSpace(r.ContactPersons[i].Name)
		r.ContactPersons[i].Email = trimOptional(r.ContactPersons[i].Email)
		if phone := r.ContactPersons[i].Phone; phone != nil {
			phone.DialCode = strings.TrimSpace(phone.DialCode)
			phone.Number = strings.TrimSpace(phone.Number)
			if phone.DialCode == "" && phone.Number == "" {
				r.ContactPersons[i].Phone = nil
			}
		}
	}
	if r.AttachmentFileID.Value != nil {
		r.AttachmentFileID.Value = trimOptional(r.AttachmentFileID.Value)
	}
}

func (r ServiceCenterWriteRequest) Validate() []CRUDFieldError {
	var errs []CRUDFieldError
	add := func(field, message string) { errs = append(errs, CRUDFieldError{Field: field, Message: message}) }

	if r.Name == "" {
		add("name", "Name must not be empty")
	} else if utf8.RuneCountInString(r.Name) > 120 {
		add("name", "Max. 120 characters")
	} else if containsEmoji(r.Name) {
		add("name", "Name contains invalid characters")
	}
	if len(r.ServiceTypes) == 0 {
		add("serviceTypes", "Select at least one service type")
	}
	for _, serviceType := range r.ServiceTypes {
		if serviceType != "Routine" && serviceType != "Repair" {
			add("serviceTypes", "Select a valid service type")
			break
		}
	}
	if len(r.AvailableTo) == 0 {
		add("availableTo", "Select at least one group")
	}
	validateIDSet := func(field string, values []string) {
		seen := make(map[string]struct{}, len(values))
		for _, value := range values {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				add(field, "Select a valid "+map[string]string{"availableTo": "group", "specializations": "category", "brands": "brand"}[field])
				break
			}
			if _, ok := seen[value]; ok {
				add(field, "Duplicate IDs are not allowed")
				break
			}
			seen[value] = struct{}{}
		}
	}
	validateIDSet("availableTo", r.AvailableTo)
	validateIDSet("specializations", r.Specializations)
	validateIDSet("brands", r.Brands)
	if r.Country == "" {
		add("country", "Country must not be empty")
	}
	if r.City == "" {
		add("city", "City must not be empty")
	} else if utf8.RuneCountInString(r.City) > 120 {
		add("city", "Max. 120 characters")
	} else if containsEmoji(r.City) {
		add("city", "City contains invalid characters")
	}
	validateOptionalText := func(field string, value *string, max int, emojiMessage string) {
		if value == nil {
			return
		}
		if utf8.RuneCountInString(*value) > max {
			add(field, "Max. "+strconv.Itoa(max)+" characters")
		} else if emojiMessage != "" && containsEmoji(*value) {
			add(field, emojiMessage)
		}
	}
	validateOptionalText("provinceState", r.ProvinceState, 120, "Province / State contains invalid characters")
	validateOptionalText("postalCode", r.PostalCode, 120, "Postal code contains invalid characters")
	validateOptionalText("addressLine", r.AddressLine, 360, "")
	validateOptionalText("notes", r.Notes, 360, "")

	companyPhonePresent := r.CompanyPhone != nil && r.CompanyPhone.Number != ""
	companyEmailPresent := r.CompanyEmail != nil && *r.CompanyEmail != ""
	if !companyPhonePresent && !companyEmailPresent {
		add("companyPhone", "Add a company phone or email")
		add("companyEmail", "Add a company phone or email")
	}
	validatePhone := func(field string, phone *ServiceCenterPhone) {
		if phone == nil || phone.Number == "" {
			return
		}
		for _, digit := range phone.Number {
			if digit < '0' || digit > '9' {
				add(field, "Enter a valid phone number")
				return
			}
		}
		if len(phone.Number) < 4 {
			add(field, "Min. 4 characters")
		} else if len(phone.Number) > 15 {
			add(field, "Max. 15 characters")
		}
		if phone.DialCode == "" {
			add(field+".dialCode", "Dial code must not be empty")
		}
	}
	validatePhone("companyPhone.number", r.CompanyPhone)
	validateEmail := func(field string, value *string) {
		if value == nil || *value == "" {
			return
		}
		address, err := mail.ParseAddress(*value)
		if err != nil || address.Address != *value {
			add(field, "Enter a valid email")
		}
		if utf8.RuneCountInString(*value) > 120 {
			add(field, "Max. 120 characters")
		}
	}
	validateEmail("companyEmail", r.CompanyEmail)
	if len(r.ContactPersons) > 3 {
		add("contactPersons", "Up to 3 contact persons, with one primary.")
	} else if len(r.ContactPersons) > 0 {
		primaryCount := 0
		for _, contact := range r.ContactPersons {
			if contact.IsPrimary {
				primaryCount++
			}
		}
		if primaryCount != 1 {
			add("contactPersons", "Up to 3 contact persons, with one primary.")
		}
	}
	for i, contact := range r.ContactPersons {
		prefix := "contactPersons." + strconv.Itoa(i)
		if contact.Name == "" {
			add(prefix+".name", "Contact name must not be empty")
		} else if utf8.RuneCountInString(contact.Name) > 120 {
			add(prefix+".name", "Max. 120 characters")
		} else if containsEmoji(contact.Name) {
			add(prefix+".name", "Contact name contains invalid characters")
		}
		hasPhone := contact.Phone != nil && contact.Phone.Number != ""
		hasEmail := contact.Email != nil && *contact.Email != ""
		if !hasPhone && !hasEmail {
			add(prefix+".phone", "Add a phone or email")
			add(prefix+".email", "Add a phone or email")
		}
		validatePhone(prefix+".phone.number", contact.Phone)
		validateEmail(prefix+".email", contact.Email)
	}
	return errs
}

func (r UpdateServiceCenterRequest) Validate() []CRUDFieldError {
	var errs []CRUDFieldError
	if !r.versionPresent {
		errs = append(errs, CRUDFieldError{Field: "version", Message: "version is required"})
	} else if r.Version < 1 {
		errs = append(errs, CRUDFieldError{Field: "version", Message: "must be greater than or equal to 1"})
	}
	return append(errs, r.ServiceCenterWriteRequest.Validate()...)
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func containsEmoji(value string) bool {
	for _, r := range value {
		if (r >= 0x1F000 && r <= 0x1FAFF) ||
			(r >= 0x2600 && r <= 0x27BF) ||
			(r >= 0x2300 && r <= 0x23FF) ||
			(r >= 0x2B00 && r <= 0x2BFF) ||
			r == 0xFE0F || r == 0x200D {
			return true
		}
	}
	return false
}

package model

import "time"

type ServiceCenter struct {
	ID                   int64     `json:"id,string" column:"service_centers.id"`
	Name                 string    `json:"name" column:"service_centers.name"`
	NameKey              string    `json:"name_key" column:"service_centers.name_key"`
	ServiceTypes         []string  `json:"service_types" column:"service_centers.service_types"`
	IsActive             bool      `json:"is_active" column:"service_centers.is_active"`
	Country              string    `json:"country" column:"service_centers.country"`
	ProvinceState        *string   `json:"province_state" column:"service_centers.province_state"`
	City                 string    `json:"city" column:"service_centers.city"`
	PostalCode           *string   `json:"postal_code" column:"service_centers.postal_code"`
	AddressLine          *string   `json:"address_line" column:"service_centers.address_line"`
	CompanyPhoneDialCode *string   `json:"company_phone_dial_code" column:"service_centers.company_phone_dial_code"`
	CompanyPhoneNumber   *string   `json:"company_phone_number" column:"service_centers.company_phone_number"`
	CompanyEmail         *string   `json:"company_email" column:"service_centers.company_email"`
	Notes                *string   `json:"notes" column:"service_centers.notes"`
	Version              int32     `json:"version" column:"service_centers.version"`
	CreatedByID          int64     `json:"created_by_id,string" column:"service_centers.created_by_id"`
	CreatedByName        string    `json:"created_by_name" column:"service_centers.created_by_name"`
	CreatedAt            time.Time `json:"created_at" column:"service_centers.created_at"`
	UpdatedAt            time.Time `json:"updated_at" column:"service_centers.updated_at"`
	ModifiedByID         *int64    `json:"modified_by_id,string" column:"service_centers.modified_by_id"`
	ModifiedByName       *string   `json:"modified_by_name" column:"service_centers.modified_by_name"`
}

type ServiceCenterGroup struct {
	ServiceCenterID int64 `json:"service_center_id,string" column:"service_center_groups.service_center_id"`
	GroupID         int64 `json:"group_id,string" column:"service_center_groups.group_id"`
}

type ServiceCenterCategory struct {
	ServiceCenterID int64 `json:"service_center_id,string" column:"service_center_categories.service_center_id"`
	CategoryID      int64 `json:"category_id,string" column:"service_center_categories.category_id"`
}

type ServiceCenterBrand struct {
	ServiceCenterID int64 `json:"service_center_id,string" column:"service_center_brands.service_center_id"`
	BrandID         int64 `json:"brand_id,string" column:"service_center_brands.brand_id"`
}

type ServiceCenterContact struct {
	ID              int64   `json:"id,string" column:"service_center_contacts.id"`
	ServiceCenterID int64   `json:"service_center_id,string" column:"service_center_contacts.service_center_id"`
	Position        int32   `json:"position" column:"service_center_contacts.position"`
	Name            string  `json:"name" column:"service_center_contacts.name"`
	PhoneDialCode   *string `json:"phone_dial_code" column:"service_center_contacts.phone_dial_code"`
	PhoneNumber     *string `json:"phone_number" column:"service_center_contacts.phone_number"`
	Email           *string `json:"email" column:"service_center_contacts.email"`
	IsPrimary       bool    `json:"is_primary" column:"service_center_contacts.is_primary"`
}

type ServiceCenterAttachment struct {
	ServiceCenterID *int64    `json:"service_center_id,string" column:"service_center_attachments.service_center_id"`
	FileID          int64     `json:"file_id,string" column:"service_center_attachments.file_id"`
	FileName        string    `json:"file_name" column:"service_center_attachments.file_name"`
	FileSize        int64     `json:"file_size" column:"service_center_attachments.file_size"`
	FileType        string    `json:"file_type" column:"service_center_attachments.file_type"`
	StoragePath     string    `json:"storage_path" column:"service_center_attachments.storage_path"`
	Staged          bool      `json:"staged" column:"service_center_attachments.staged"`
	CreatedAt       time.Time `json:"created_at" column:"service_center_attachments.created_at"`
}

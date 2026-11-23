CREATE TABLE service_centers (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL CHECK (name = BTRIM(name) AND CHAR_LENGTH(name) BETWEEN 1 AND 120),
    name_key TEXT NOT NULL UNIQUE CHECK (name_key = LOWER(BTRIM(name))),
    service_types TEXT[] NOT NULL CHECK (
        ARRAY_NDIMS(service_types) = 1
        AND CARDINALITY(service_types) >= 1
        AND service_types <@ ARRAY['Routine', 'Repair']::TEXT[]
    ),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    country TEXT NOT NULL,
    province_state TEXT CHECK (CHAR_LENGTH(province_state) <= 120),
    city TEXT NOT NULL CHECK (CHAR_LENGTH(city) BETWEEN 1 AND 120),
    postal_code TEXT CHECK (CHAR_LENGTH(postal_code) <= 120),
    address_line TEXT CHECK (CHAR_LENGTH(address_line) <= 360),
    company_phone_dial_code TEXT,
    company_phone_number TEXT CHECK (company_phone_number ~ '^[0-9]{1,15}$'),
    company_email TEXT CHECK (CHAR_LENGTH(company_email) <= 120),
    notes TEXT CHECK (CHAR_LENGTH(notes) <= 360),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by_id BIGINT NOT NULL,
    created_by_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_by_id BIGINT,
    modified_by_name TEXT
);

CREATE TABLE service_center_groups (
    service_center_id BIGINT NOT NULL REFERENCES service_centers (id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL,
    PRIMARY KEY (service_center_id, group_id)
);

CREATE TABLE service_center_categories (
    service_center_id BIGINT NOT NULL REFERENCES service_centers (id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL,
    PRIMARY KEY (service_center_id, category_id)
);

CREATE TABLE service_center_brands (
    service_center_id BIGINT NOT NULL REFERENCES service_centers (id) ON DELETE CASCADE,
    brand_id BIGINT NOT NULL,
    PRIMARY KEY (service_center_id, brand_id)
);

CREATE TABLE service_center_contacts (
    id BIGINT PRIMARY KEY,
    service_center_id BIGINT NOT NULL REFERENCES service_centers (id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 2),
    name TEXT NOT NULL CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 120),
    phone_dial_code TEXT,
    phone_number TEXT,
    email TEXT CHECK (CHAR_LENGTH(email) <= 120),
    is_primary BOOLEAN NOT NULL,
    UNIQUE (service_center_id, position)
);

CREATE UNIQUE INDEX idx_service_center_contacts_primary
    ON service_center_contacts (service_center_id) WHERE is_primary;

CREATE TABLE service_center_attachments (
    service_center_id BIGINT REFERENCES service_centers (id) ON DELETE CASCADE,
    file_id BIGINT PRIMARY KEY,
    file_name TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size BETWEEN 0 AND 1048576),
    file_type TEXT NOT NULL CHECK (file_type IN ('application/pdf', 'image/jpeg', 'image/png')),
    storage_path TEXT NOT NULL,
    staged BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (staged = (service_center_id IS NULL)),
    -- Replacement can attach the new file before deleting the previous row.
    UNIQUE (service_center_id) DEFERRABLE INITIALLY DEFERRED
);

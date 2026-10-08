-- Tenant singleton; consumer snapshots and analytics logs are not stored here.
CREATE TABLE IF NOT EXISTS transaction_settings (
    id BIGINT PRIMARY KEY DEFAULT 1,
    default_borrow_time INTEGER NOT NULL DEFAULT 7,
    max_borrow_time INTEGER NOT NULL DEFAULT 30,
    confirmation_email_timeout INTEGER NOT NULL DEFAULT 24,
    extension_default_duration INTEGER NOT NULL DEFAULT 7,
    max_extension_duration INTEGER NOT NULL DEFAULT 30,
    max_extension_count INTEGER NOT NULL DEFAULT 3,
    is_overdue_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_scan_at_return_required BOOLEAN NOT NULL DEFAULT FALSE,
    borrow_transaction_timeout INTEGER NOT NULL DEFAULT 3,
    max_transfer_transit_time INTEGER NOT NULL DEFAULT 3,
    is_scan_at_arrival_required BOOLEAN NOT NULL DEFAULT FALSE,
    is_transfer_document_generation_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    transfer_transaction_timeout INTEGER NOT NULL DEFAULT 5,
    is_documentation_required BOOLEAN NOT NULL DEFAULT FALSE,
    disposal_transaction_timeout INTEGER NOT NULL DEFAULT 7,
    disposal_finance_threshold NUMERIC(18,2) NOT NULL DEFAULT 10000000.00,
    is_damaged_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    damaged_reminder_interval INTEGER NOT NULL DEFAULT 7,
    is_missing_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    missing_reminder_interval INTEGER NOT NULL DEFAULT 7,
    audit_grace_period INTEGER NOT NULL DEFAULT 2,
    is_audit_geolocation_required BOOLEAN NOT NULL DEFAULT TRUE,
    modified_by_id BIGINT,
    modified_by_name VARCHAR,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_transaction_settings_singleton CHECK (id = 1),
    CONSTRAINT chk_transaction_settings_borrow_duration
        CHECK (default_borrow_time <= max_borrow_time),
    CONSTRAINT chk_transaction_settings_extension_duration
        CHECK (extension_default_duration <= max_extension_duration),
    CONSTRAINT chk_transaction_settings_max_borrow_time
        CHECK (max_borrow_time BETWEEN 1 AND 365),
    CONSTRAINT chk_transaction_settings_confirmation_email_timeout
        CHECK (confirmation_email_timeout BETWEEN 1 AND 72),
    CONSTRAINT chk_transaction_settings_max_extension_count
        CHECK (max_extension_count BETWEEN 1 AND 10),
    CONSTRAINT chk_transaction_settings_max_transfer_transit_time
        CHECK (max_transfer_transit_time BETWEEN 1 AND 365),
    CONSTRAINT chk_transaction_settings_borrow_transaction_timeout
        CHECK (borrow_transaction_timeout BETWEEN 1 AND 365),
    CONSTRAINT chk_transaction_settings_transfer_transaction_timeout
        CHECK (transfer_transaction_timeout BETWEEN 1 AND 365),
    CONSTRAINT chk_transaction_settings_disposal_transaction_timeout
        CHECK (disposal_transaction_timeout BETWEEN 1 AND 365),
    CONSTRAINT chk_transaction_settings_disposal_finance_threshold
        CHECK (disposal_finance_threshold >= 0),
    CONSTRAINT chk_transaction_settings_damaged_reminder_interval
        CHECK (damaged_reminder_interval BETWEEN 1 AND 90),
    CONSTRAINT chk_transaction_settings_missing_reminder_interval
        CHECK (missing_reminder_interval BETWEEN 1 AND 90),
    CONSTRAINT chk_transaction_settings_audit_grace_period
        CHECK (audit_grace_period BETWEEN 1 AND 30)
);

INSERT INTO transaction_settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

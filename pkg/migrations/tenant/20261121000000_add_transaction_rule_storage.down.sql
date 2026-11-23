DROP INDEX IF EXISTS idx_transaction_rule_overrides_list;
DROP TABLE IF EXISTS transaction_rule_overrides;
DROP TABLE IF EXISTS transaction_rule_fields;
ALTER TABLE transaction_settings DROP COLUMN IF EXISTS settings_version;

ALTER TABLE transaction_settings
    ADD COLUMN IF NOT EXISTS default_borrow_time INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS max_borrow_time INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS confirmation_email_timeout INTEGER NOT NULL DEFAULT 24,
    ADD COLUMN IF NOT EXISTS extension_default_duration INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS max_extension_duration INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS max_extension_count INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS is_overdue_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS is_scan_at_return_required BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS borrow_transaction_timeout INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS max_transfer_transit_time INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS is_scan_at_arrival_required BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_transfer_document_generation_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS transfer_transaction_timeout INTEGER NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS is_documentation_required BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS disposal_transaction_timeout INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS disposal_finance_threshold NUMERIC(18,2) NOT NULL DEFAULT 10000000.00,
    ADD COLUMN IF NOT EXISTS is_damaged_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS damaged_reminder_interval INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS is_missing_reminder_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS missing_reminder_interval INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS audit_grace_period INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS is_audit_geolocation_required BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE transaction_settings
    ADD CONSTRAINT chk_transaction_settings_borrow_duration CHECK (default_borrow_time <= max_borrow_time),
    ADD CONSTRAINT chk_transaction_settings_extension_duration CHECK (extension_default_duration <= max_extension_duration),
    ADD CONSTRAINT chk_transaction_settings_max_borrow_time CHECK (max_borrow_time BETWEEN 1 AND 365),
    ADD CONSTRAINT chk_transaction_settings_confirmation_email_timeout CHECK (confirmation_email_timeout BETWEEN 1 AND 72),
    ADD CONSTRAINT chk_transaction_settings_max_extension_count CHECK (max_extension_count BETWEEN 1 AND 10),
    ADD CONSTRAINT chk_transaction_settings_max_transfer_transit_time CHECK (max_transfer_transit_time BETWEEN 1 AND 365),
    ADD CONSTRAINT chk_transaction_settings_borrow_transaction_timeout CHECK (borrow_transaction_timeout BETWEEN 1 AND 365),
    ADD CONSTRAINT chk_transaction_settings_transfer_transaction_timeout CHECK (transfer_transaction_timeout BETWEEN 1 AND 365),
    ADD CONSTRAINT chk_transaction_settings_disposal_transaction_timeout CHECK (disposal_transaction_timeout BETWEEN 1 AND 365),
    ADD CONSTRAINT chk_transaction_settings_disposal_finance_threshold CHECK (disposal_finance_threshold >= 0),
    ADD CONSTRAINT chk_transaction_settings_damaged_reminder_interval CHECK (damaged_reminder_interval BETWEEN 1 AND 90),
    ADD CONSTRAINT chk_transaction_settings_missing_reminder_interval CHECK (missing_reminder_interval BETWEEN 1 AND 90),
    ADD CONSTRAINT chk_transaction_settings_audit_grace_period CHECK (audit_grace_period BETWEEN 1 AND 30);

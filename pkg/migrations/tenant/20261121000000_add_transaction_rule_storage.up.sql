ALTER TABLE transaction_settings
    ADD COLUMN settings_version BIGINT NOT NULL DEFAULT 1 CHECK (settings_version >= 1);

-- Rows with NULL category_id are the authoritative Company default; the flat singleton columns on
-- transaction_settings are dropped at the end of this migration. Company-only fields (Audit and Geolocation tabs, Damaged & Missing,
-- Change Cost Center with Group, Transaction timeout, Send email notifications) never have Category rows.
CREATE TABLE transaction_rule_fields (
    id BIGINT PRIMARY KEY,
    category_id BIGINT,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN (
        'borrow', 'assign', 'transfer', 'disposal', 'damaged_missing', 'audit', 'maintenance'
    )),
    field_key TEXT NOT NULL CHECK (field_key <> ''),
    mode TEXT NOT NULL,
    value JSONB,
    modified_by_id BIGINT,
    modified_by_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (category_id, transaction_type, field_key),
    CHECK (
        (mode = 'inherit' AND category_id IS NOT NULL AND value IS NULL)
        OR (mode = 'own' AND value IS NOT NULL
            AND jsonb_typeof(value) IN ('number', 'boolean', 'string', 'array'))
    ),
    CHECK (
        category_id IS NULL OR (
            transaction_type NOT IN ('damaged_missing', 'audit')
            AND field_key NOT IN (
                'change_cost_center_with_group', 'transaction_timeout', 'send_email_notifications', 'require_location'
            )
        )
    )
);

-- Company default seed. Ids 1..N are deterministic and cannot collide with Snowflake ids.
INSERT INTO transaction_rule_fields (id, category_id, transaction_type, field_key, mode, value)
SELECT ROW_NUMBER() OVER (ORDER BY v.transaction_type, v.field_key), NULL, v.transaction_type, v.field_key, 'own', v.value
FROM (VALUES
        ('borrow', 'allow_borrow', 'true'::jsonb),
        ('borrow', 'duration_unit', '"days"'::jsonb),
        ('borrow', 'max_duration', '14'::jsonb),
        ('borrow', 'max_extensions', '1'::jsonb),
        ('borrow', 'max_assets_per_borrower', '3'::jsonb),
        ('borrow', 'allowed_borrower', '"employee_in_any_group"'::jsonb),
        ('borrow', 'block_new_borrow_when_overdue', 'true'::jsonb),
        ('borrow', 'require_approval', 'true'::jsonb),
        ('borrow', 'self_service', 'false'::jsonb),
        ('borrow', 'auto_approve', 'true'::jsonb),
        ('borrow', 'auto_approve_max_duration', '3'::jsonb),
        ('borrow', 'approval_type', '"any_one_approver"'::jsonb),
        ('borrow', 'due_date_reminders', '"d_3_d_1_d_day"'::jsonb),
        ('borrow', 'photo_at_start', 'true'::jsonb),
        ('borrow', 'photo_at_end', 'true'::jsonb),
        ('borrow', 'transaction_timeout', '7'::jsonb),
        ('borrow', 'send_email_notifications', 'true'::jsonb),
        ('assign', 'allow_assign', 'true'::jsonb),
        ('assign', 'allowed_assignee_employee', 'true'::jsonb),
        ('assign', 'allowed_assignee_division', 'true'::jsonb),
        ('assign', 'max_assets_per_assignee', '0'::jsonb),
        ('assign', 'need_recipient_confirmation', 'true'::jsonb),
        ('assign', 'require_approval', 'false'::jsonb),
        ('assign', 'approval_type', '"any_one_approver"'::jsonb),
        ('assign', 'photo_at_start', 'true'::jsonb),
        ('assign', 'photo_at_end', 'true'::jsonb),
        ('assign', 'transaction_timeout', '7'::jsonb),
        ('assign', 'send_email_notifications', 'false'::jsonb),
        ('transfer', 'allow_transfer', 'true'::jsonb),
        ('transfer', 'destination_scope', '"any_group"'::jsonb),
        ('transfer', 'change_cost_center_with_group', '"never"'::jsonb),
        ('transfer', 'in_transit_overdue_after', '7'::jsonb),
        ('transfer', 'destination_quota_check', 'true'::jsonb),
        ('transfer', 'require_approval', 'true'::jsonb),
        ('transfer', 'auto_approve', 'true'::jsonb),
        ('transfer', 'auto_approve_same_parent_group', 'true'::jsonb),
        ('transfer', 'approval_type', '"any_one_approver"'::jsonb),
        ('transfer', 'approval_scope', '"both"'::jsonb),
        ('transfer', 'photo_at_start', 'true'::jsonb),
        ('transfer', 'photo_at_end', 'true'::jsonb),
        ('transfer', 'transaction_timeout', '7'::jsonb),
        ('transfer', 'send_email_notifications', 'false'::jsonb),
        ('disposal', 'allow_disposal', 'true'::jsonb),
        ('disposal', 'allowed_disposal_types', '["sell","scrap","donate","trade_in","write_off"]'::jsonb),
        ('disposal', 'min_asset_age_warning', '12'::jsonb),
        ('disposal', 'max_deviation_from_book_value', '30'::jsonb),
        ('disposal', 'require_approval', 'true'::jsonb),
        ('disposal', 'approval_type', '"any_one_approver"'::jsonb),
        ('disposal', 'transaction_timeout', '7'::jsonb),
        ('disposal', 'send_email_notifications', 'false'::jsonb),
        ('damaged_missing', 'long_missing_after', '30'::jsonb),
        ('damaged_missing', 'after_photo_required', 'false'::jsonb),
        ('audit', 'require_approval', 'true'::jsonb),
        ('audit', 'approval_type', '"any_one_approver"'::jsonb),
        ('audit', 'transaction_timeout', '7'::jsonb),
        ('audit', 'send_email_notifications', 'false'::jsonb),
        ('audit', 'grace_period', '3'::jsonb),
        ('audit', 'require_location', 'false'::jsonb),
        ('maintenance', 'approval_type', '"any_one_approver"'::jsonb),
        ('maintenance', 'reminder_before_due', '3'::jsonb),
        ('maintenance', 'photo_after', 'true'::jsonb),
        ('maintenance', 'require_approval_to_close', 'false'::jsonb),
        ('maintenance', 'require_scan_to_start', 'false'::jsonb),
        ('maintenance', 'require_location', 'false'::jsonb)
) AS v (transaction_type, field_key, value)
ON CONFLICT DO NOTHING;

-- NULL category_id represents an All categories override. Deleted overrides are removed.
-- target_name, category_path and target_group_id are snapshots taken at creation.
CREATE TABLE transaction_rule_overrides (
    id BIGINT PRIMARY KEY,
    category_id BIGINT,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('borrow', 'assign', 'transfer', 'disposal')),
    scope TEXT NOT NULL CHECK (scope IN ('group', 'asset_name', 'asset')),
    target_id BIGINT NOT NULL,
    target_name TEXT NOT NULL,
    category_path TEXT,
    target_group_id BIGINT,
    fields JSONB NOT NULL CHECK (jsonb_typeof(fields) = 'object' AND fields <> '{}'::jsonb),
    created_by_id BIGINT NOT NULL,
    created_by_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (scope, target_id, category_id, transaction_type)
);

CREATE INDEX idx_transaction_rule_overrides_list
    ON transaction_rule_overrides (transaction_type, category_id, created_at DESC, id);

-- Rule values live in transaction_rule_fields (Company default = NULL category_id) and
-- transaction_rule_overrides. The flat singleton columns are unused by the settings-transaction-rules
-- module; transaction_settings keeps only id, settings_version, modified_by_* and timestamps.
-- Dropping a column also drops the CHECK constraints that reference it.
ALTER TABLE transaction_settings
    DROP COLUMN IF EXISTS default_borrow_time,
    DROP COLUMN IF EXISTS max_borrow_time,
    DROP COLUMN IF EXISTS confirmation_email_timeout,
    DROP COLUMN IF EXISTS extension_default_duration,
    DROP COLUMN IF EXISTS max_extension_duration,
    DROP COLUMN IF EXISTS max_extension_count,
    DROP COLUMN IF EXISTS is_overdue_reminder_enabled,
    DROP COLUMN IF EXISTS is_scan_at_return_required,
    DROP COLUMN IF EXISTS borrow_transaction_timeout,
    DROP COLUMN IF EXISTS max_transfer_transit_time,
    DROP COLUMN IF EXISTS is_scan_at_arrival_required,
    DROP COLUMN IF EXISTS is_transfer_document_generation_enabled,
    DROP COLUMN IF EXISTS transfer_transaction_timeout,
    DROP COLUMN IF EXISTS is_documentation_required,
    DROP COLUMN IF EXISTS disposal_transaction_timeout,
    DROP COLUMN IF EXISTS disposal_finance_threshold,
    DROP COLUMN IF EXISTS is_damaged_reminder_enabled,
    DROP COLUMN IF EXISTS damaged_reminder_interval,
    DROP COLUMN IF EXISTS is_missing_reminder_enabled,
    DROP COLUMN IF EXISTS missing_reminder_interval,
    DROP COLUMN IF EXISTS audit_grace_period,
    DROP COLUMN IF EXISTS is_audit_geolocation_required;

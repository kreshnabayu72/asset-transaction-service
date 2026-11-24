-- Damage records (db-gaps G-1), Damage Log nodes (G-2) and Report Damaged request tokens, in dependency order.
CREATE TABLE damage_reports (
    id BIGINT PRIMARY KEY,
    asset_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN (
        'waiting_for_confirmation', 'damaged', 'reported_disposal', 'in_disposal_process',
        'repaired', 'disposed', 'rejected', 'canceled'
    )),
    source VARCHAR(32) NOT NULL CHECK (source IN (
        'Manual', 'Reported by scan', 'Reported by holder', 'Borrowed asset', 'Borrow return',
        'Assigned asset', 'Assign return', 'Stock opname', 'Transfer receive', 'Missing asset', 'Registration'
    )),
    reporter_type VARCHAR(16) NOT NULL CHECK (reporter_type IN ('admin', 'general')),
    effect VARCHAR(16) NOT NULL CHECK (effect IN ('status_damaged', 'marker_only')),
    missing_marker VARCHAR(32) CHECK (missing_marker IN ('reported_missing', 'missing')),
    what_happened VARCHAR(360),
    last_holder_id BIGINT,
    last_holder_full_name VARCHAR(255),
    last_holder_first_name VARCHAR(255),
    last_holder_last_name VARCHAR(255),
    last_holder_division VARCHAR(255),
    last_holder_employee_id BIGINT,
    last_holder_pic_name VARCHAR(255),
    reported_by_id BIGINT NOT NULL,
    reported_by_full_name VARCHAR(255) NOT NULL,
    reported_by_first_name VARCHAR(255) NOT NULL,
    reported_by_last_name VARCHAR(255) NOT NULL,
    on_behalf_of_employee_id BIGINT,
    on_behalf_of_name VARCHAR(255),
    reported_at TIMESTAMPTZ NOT NULL,
    last_node_by_id BIGINT,
    last_node_by_name VARCHAR(255),
    last_node_at TIMESTAMPTZ,
    disposal_report_id BIGINT,
    resolved_by_id BIGINT,
    resolved_by_name VARCHAR(255),
    resolved_at TIMESTAMPTZ,
    resolved_group_id BIGINT,
    resolved_group_name VARCHAR(255),
    resolved_category_id BIGINT,
    resolved_category_name VARCHAR(255),
    resolved_commercial_value JSONB,
    resolved_fiscal_value JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_damage_reports_running_asset
    ON damage_reports (asset_id)
    WHERE status IN ('waiting_for_confirmation', 'damaged', 'reported_disposal', 'in_disposal_process');

CREATE INDEX idx_damage_reports_status ON damage_reports (status);

CREATE INDEX idx_damage_reports_reported_by_id ON damage_reports (reported_by_id);

CREATE TABLE damage_report_nodes (
    id BIGINT PRIMARY KEY,
    record_id BIGINT NOT NULL REFERENCES damage_reports (id) ON DELETE RESTRICT,
    node_type VARCHAR(40) NOT NULL CHECK (node_type IN (
        'reported', 'linked', 'confirmed', 'rejected', 'canceled', 'missing_reported', 'asset_found',
        'missing_closed', 'repaired', 'disposal_reported', 'disposal_report_declined',
        'disposal_report_canceled', 'disposal_requested', 'disposal_rejected', 'disposal_canceled',
        'disposal_removed', 'disposed'
    )),
    actor_id BIGINT,
    actor_name VARCHAR(255),
    on_behalf_of_name VARCHAR(255),
    what_happened VARCHAR(360),
    reason VARCHAR(360),
    repair_note VARCHAR(360),
    note VARCHAR(360),
    damaged_photo_file_id BIGINT,
    after_photo_file_id BIGINT,
    photo_deleted_at TIMESTAMPTZ,
    manual_identification BOOLEAN NOT NULL DEFAULT FALSE,
    tag_report_id BIGINT,
    report_id VARCHAR(64),
    disposal_no VARCHAR(64),
    origin_ref_type VARCHAR(32),
    origin_ref_no VARCHAR(64),
    event_id VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_damage_report_nodes_event_id
    ON damage_report_nodes (event_id)
    WHERE event_id IS NOT NULL;

CREATE TABLE damage_report_request_tokens (
    token VARCHAR(64) NOT NULL,
    actor_id BIGINT NOT NULL,
    request_hash CHAR(64) NOT NULL,
    response JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_id, token)
);

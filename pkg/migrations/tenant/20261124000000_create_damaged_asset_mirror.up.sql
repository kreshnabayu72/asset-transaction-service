-- Read copy of master-asset-registry-service assets: columns per the damaged LLD (common invariants), types and CHECKs per MAR's effective migrations.
CREATE TABLE assets (
    id BIGINT PRIMARY KEY,
    asset_code VARCHAR(64),
    asset_name_id BIGINT,
    asset_name VARCHAR,
    asset_increment BIGINT,
    asset_name_model_id BIGINT,
    brand_id BIGINT,
    brand_name VARCHAR(255),
    division_id BIGINT,
    division_name VARCHAR,
    group_id BIGINT,
    group_name VARCHAR,
    purchase_date DATE,
    purchase_price NUMERIC(18, 2),
    warranty_expiry DATE,
    lifecycle_status VARCHAR(32) NOT NULL DEFAULT 'draft',
    custody_status VARCHAR(32) NOT NULL DEFAULT 'available',
    holder_id BIGINT NULL,
    condition VARCHAR(32) NOT NULL DEFAULT 'normal',
    registered_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_assets_lifecycle_status
        CHECK (lifecycle_status IN ('draft', 'waiting_for_approval', 'active', 'disposed', 'deleted')),
    CONSTRAINT chk_assets_custody_status
        CHECK (custody_status IN ('available', 'in_borrow_process', 'borrowed', 'in_assignment_process', 'assigned', 'in_transfer_process')),
    CONSTRAINT chk_assets_condition
        CHECK (condition IN ('normal', 'damaged', 'missing', 'in_repair', 'in_tracking', 'permanently_damaged',
                             'permanently_missing', 'reported_disposal', 'in_disposal_process')),
    CONSTRAINT chk_assets_asset_code_required
        CHECK (asset_code IS NOT NULL OR lifecycle_status IN ('draft', 'waiting_for_approval')),
    CONSTRAINT chk_assets_registered_at
        CHECK (lifecycle_status NOT IN ('active', 'disposed') OR registered_at IS NOT NULL),
    CONSTRAINT chk_assets_purchase_price CHECK (purchase_price IS NULL OR purchase_price >= 0)
);

CREATE UNIQUE INDEX uq_assets_asset_code ON assets (asset_code);

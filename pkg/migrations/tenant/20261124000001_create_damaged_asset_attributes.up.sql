-- Transaction-scoped display attributes per asset (ADR 017 section 11, ADR 007); the damaged module uses transaction_type = 'damaged'.
CREATE TABLE asset_attributes (
    id BIGINT PRIMARY KEY,
    asset_id BIGINT NOT NULL,
    transaction_type VARCHAR(255) NOT NULL,
    name_with_sequence VARCHAR(255),
    brand_name VARCHAR(255),
    model_name VARCHAR(255),
    category_name VARCHAR(255),
    alias_name VARCHAR(255),
    status VARCHAR(255),
    group_id BIGINT,
    group_name VARCHAR(255),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (asset_id, transaction_type)
);

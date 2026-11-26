-- be-core-utils InsertBuilder stamps updated_at on every insert; the override table must have the column.
ALTER TABLE transaction_rule_overrides ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

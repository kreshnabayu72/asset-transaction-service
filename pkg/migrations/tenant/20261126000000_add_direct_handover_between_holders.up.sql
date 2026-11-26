-- Company default for the Assign direct handover field (Allow).
INSERT INTO transaction_rule_fields (id, category_id, transaction_type, field_key, mode, value)
SELECT COALESCE(MAX(id), 0) + 1, NULL, 'assign', 'direct_handover_between_holders', 'own', 'true'::jsonb
FROM transaction_rule_fields;

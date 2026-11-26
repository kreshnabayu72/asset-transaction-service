DELETE FROM transaction_rule_fields
WHERE category_id IS NULL AND transaction_type = 'assign' AND field_key = 'direct_handover_between_holders';

ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_by_customer_id;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_to_supplier_id;

ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS paid_by VARCHAR(100);
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS paid_to VARCHAR(100);

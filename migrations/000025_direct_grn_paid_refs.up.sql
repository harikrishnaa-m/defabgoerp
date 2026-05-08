-- Replace paid_by/paid_to VARCHAR with proper FK references
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_by;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_to;

ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS paid_by_customer_id UUID REFERENCES customers(id) ON DELETE SET NULL;

ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS paid_to_supplier_id UUID REFERENCES suppliers(id) ON DELETE SET NULL;

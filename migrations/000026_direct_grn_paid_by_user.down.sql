ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_by_user_id;

ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS paid_by_customer_id UUID REFERENCES customers(id) ON DELETE SET NULL;

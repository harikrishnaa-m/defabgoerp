-- Correct paid_by to reference users (internal staff) instead of customers
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_by_customer_id;

ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS paid_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

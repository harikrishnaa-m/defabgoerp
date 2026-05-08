-- Direct GRN: new columns and table

-- purchase_orders: purchase type (e.g. "Loc Purchase", "Outstation")
ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS purchase_type VARCHAR(50);

-- goods_receipts: transport and lorry receipt number
ALTER TABLE goods_receipts ADD COLUMN IF NOT EXISTS transport VARCHAR(100);
ALTER TABLE goods_receipts ADD COLUMN IF NOT EXISTS lr_number VARCHAR(100);

-- purchase_order_items: additional tracking fields
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS product_code VARCHAR(100);
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS category VARCHAR(100);
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS free_qty NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS additional_work TEXT;
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS additional_work_amount NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS paid_by VARCHAR(100);
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS paid_to VARCHAR(100);
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS cash_amount NUMERIC NOT NULL DEFAULT 0;
ALTER TABLE purchase_order_items ADD COLUMN IF NOT EXISTS credit_amount NUMERIC NOT NULL DEFAULT 0;

-- purchase_charges: freight, coolie, handling, etc.
CREATE TABLE IF NOT EXISTS purchase_charges (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    charge_type   VARCHAR(100) NOT NULL,
    amount        NUMERIC NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE variants
    ADD COLUMN IF NOT EXISTS wholesale_price DECIMAL(10, 2);

ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS selling_price   DECIMAL(10, 2),
    ADD COLUMN IF NOT EXISTS wholesale_price DECIMAL(10, 2);

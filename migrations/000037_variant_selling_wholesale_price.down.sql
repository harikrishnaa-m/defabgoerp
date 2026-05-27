ALTER TABLE purchase_order_items
    DROP COLUMN IF EXISTS wholesale_price,
    DROP COLUMN IF EXISTS selling_price;

ALTER TABLE variants
    DROP COLUMN IF EXISTS wholesale_price;

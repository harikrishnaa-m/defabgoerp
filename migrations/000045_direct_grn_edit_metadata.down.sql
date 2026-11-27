ALTER TABLE purchase_order_items
    DROP COLUMN IF EXISTS category_id,
    DROP COLUMN IF EXISTS product_id,
    DROP COLUMN IF EXISTS create_variant;

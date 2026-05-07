-- Add product_code to raw_material_stocks
ALTER TABLE raw_material_stocks ADD COLUMN IF NOT EXISTS product_code VARCHAR(100);

-- Drop the single unique constraint and replace with two partial ones:
-- 1. When product_code is present, uniqueness is by (product_code, warehouse_id)
-- 2. When product_code is absent, keep the original (item_name, warehouse_id) behaviour
ALTER TABLE raw_material_stocks DROP CONSTRAINT IF EXISTS raw_material_stocks_item_name_warehouse_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS raw_material_stocks_product_code_warehouse_key
    ON raw_material_stocks (product_code, warehouse_id)
    WHERE product_code IS NOT NULL AND product_code <> '';

CREATE UNIQUE INDEX IF NOT EXISTS raw_material_stocks_item_name_warehouse_key
    ON raw_material_stocks (item_name, warehouse_id)
    WHERE product_code IS NULL OR product_code = '';

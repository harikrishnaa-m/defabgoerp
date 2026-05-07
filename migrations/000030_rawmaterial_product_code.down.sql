DROP INDEX IF EXISTS raw_material_stocks_product_code_warehouse_key;
DROP INDEX IF EXISTS raw_material_stocks_item_name_warehouse_key;
ALTER TABLE raw_material_stocks DROP COLUMN IF EXISTS product_code;
ALTER TABLE raw_material_stocks ADD CONSTRAINT raw_material_stocks_item_name_warehouse_id_key UNIQUE (item_name, warehouse_id);

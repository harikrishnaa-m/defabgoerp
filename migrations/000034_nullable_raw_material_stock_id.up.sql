-- raw_material_stock_id must be nullable now that variant_id is an alternative input.
-- Exactly one of (raw_material_stock_id, variant_id) should be set per row.

ALTER TABLE job_order_materials
    ALTER COLUMN raw_material_stock_id DROP NOT NULL;

ALTER TABLE production_materials
    ALTER COLUMN raw_material_stock_id DROP NOT NULL;

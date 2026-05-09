DROP INDEX IF EXISTS variants_variant_code_idx;

ALTER TABLE production_materials
    DROP COLUMN IF EXISTS variant_warehouse_id,
    DROP COLUMN IF EXISTS variant_id;

ALTER TABLE job_order_materials
    DROP COLUMN IF EXISTS variant_warehouse_id,
    DROP COLUMN IF EXISTS variant_id;

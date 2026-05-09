-- Add variant_id to job_order_materials so job orders can consume product variants
ALTER TABLE job_order_materials
    ADD COLUMN IF NOT EXISTS variant_id UUID REFERENCES variants(id),
    ADD COLUMN IF NOT EXISTS variant_warehouse_id UUID REFERENCES warehouses(id);
-- raw_material_stock_id stays (backward compat for old rows)
-- Exactly one of (raw_material_stock_id, variant_id) should be set on new rows.

-- Add variant_id to production_materials so production can consume product variants
ALTER TABLE production_materials
    ADD COLUMN IF NOT EXISTS variant_id UUID REFERENCES variants(id),
    ADD COLUMN IF NOT EXISTS variant_warehouse_id UUID REFERENCES warehouses(id);
-- raw_material_stock_id stays (backward compat for old rows)

-- Index for fast variant_code lookup (used by GRN stock routing)
CREATE INDEX IF NOT EXISTS variants_variant_code_idx ON variants(variant_code);

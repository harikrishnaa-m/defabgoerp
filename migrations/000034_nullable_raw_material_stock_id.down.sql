ALTER TABLE job_order_materials
    ALTER COLUMN raw_material_stock_id SET NOT NULL;

ALTER TABLE production_materials
    ALTER COLUMN raw_material_stock_id SET NOT NULL;

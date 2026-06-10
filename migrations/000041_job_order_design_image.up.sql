ALTER TABLE job_orders
    ADD COLUMN IF NOT EXISTS design_image_url VARCHAR(500);

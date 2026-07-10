ALTER TABLE job_order_items
    DROP COLUMN IF EXISTS designer_name,
    DROP COLUMN IF EXISTS cutter_name,
    DROP COLUMN IF EXISTS stitcher_name;

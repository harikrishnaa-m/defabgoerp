ALTER TABLE job_order_items
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS sub_category,
    DROP COLUMN IF EXISTS pieces;

ALTER TABLE job_order_items
    ALTER COLUMN description SET NOT NULL;

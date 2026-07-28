DROP TABLE IF EXISTS job_order_item_work_logs;

ALTER TABLE job_order_items
    DROP COLUMN IF EXISTS hand_worker_name;

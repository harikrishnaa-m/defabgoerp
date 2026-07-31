-- Reverse: restore free-text name columns from the worker registry, then drop it.

ALTER TABLE job_order_item_work_logs
    ADD COLUMN IF NOT EXISTS worker_name VARCHAR(100) DEFAULT '';

UPDATE job_order_item_work_logs wl SET worker_name = w.name
    FROM job_order_workers w
    WHERE wl.worker_id = w.id;

ALTER TABLE job_order_item_work_logs
    DROP COLUMN IF EXISTS worker_id;

ALTER TABLE job_order_items
    ADD COLUMN IF NOT EXISTS designer_name VARCHAR(100) DEFAULT '',
    ADD COLUMN IF NOT EXISTS cutter_name VARCHAR(100) DEFAULT '',
    ADD COLUMN IF NOT EXISTS stitcher_name VARCHAR(100) DEFAULT '',
    ADD COLUMN IF NOT EXISTS hand_worker_name VARCHAR(100) DEFAULT '';

UPDATE job_order_items it SET designer_name = w.name
    FROM job_order_workers w WHERE it.designer_id = w.id;
UPDATE job_order_items it SET cutter_name = w.name
    FROM job_order_workers w WHERE it.cutter_id = w.id;
UPDATE job_order_items it SET stitcher_name = w.name
    FROM job_order_workers w WHERE it.stitcher_id = w.id;
UPDATE job_order_items it SET hand_worker_name = w.name
    FROM job_order_workers w WHERE it.hand_worker_id = w.id;

ALTER TABLE job_order_items
    DROP COLUMN IF EXISTS designer_id,
    DROP COLUMN IF EXISTS cutter_id,
    DROP COLUMN IF EXISTS stitcher_id,
    DROP COLUMN IF EXISTS hand_worker_id;

DROP TABLE IF EXISTS job_order_workers;

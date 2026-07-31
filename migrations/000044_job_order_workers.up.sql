-- Job order workers: a real worker registry (cutter/stitcher/hand worker/designer)
-- replacing free-text staff name columns with relational IDs, so time-tracked
-- work (job_order_item_work_logs) and staff assignments (job_order_items)
-- both reference a single worker record. Role is an optional tag only —
-- a worker with role = NULL can be assigned to any field.

CREATE TABLE IF NOT EXISTS job_order_workers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    phone      VARCHAR(20) DEFAULT '',
    role       VARCHAR(20),  -- optional tag: DESIGNER, CUTTER, STITCHER, HAND_WORKER
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_order_workers_role ON job_order_workers(role);

-- Backfill: one worker row per distinct trimmed name found across the old free-text columns
INSERT INTO job_order_workers (name)
SELECT DISTINCT TRIM(nm) FROM (
    SELECT designer_name AS nm FROM job_order_items WHERE COALESCE(designer_name,'') <> ''
    UNION
    SELECT cutter_name FROM job_order_items WHERE COALESCE(cutter_name,'') <> ''
    UNION
    SELECT stitcher_name FROM job_order_items WHERE COALESCE(stitcher_name,'') <> ''
    UNION
    SELECT hand_worker_name FROM job_order_items WHERE COALESCE(hand_worker_name,'') <> ''
    UNION
    SELECT worker_name FROM job_order_item_work_logs WHERE COALESCE(worker_name,'') <> ''
) names
WHERE TRIM(nm) <> '';

-- job_order_items: add FK columns, backfill from names, then drop the old string columns
ALTER TABLE job_order_items
    ADD COLUMN IF NOT EXISTS designer_id    UUID REFERENCES job_order_workers(id),
    ADD COLUMN IF NOT EXISTS cutter_id      UUID REFERENCES job_order_workers(id),
    ADD COLUMN IF NOT EXISTS stitcher_id    UUID REFERENCES job_order_workers(id),
    ADD COLUMN IF NOT EXISTS hand_worker_id UUID REFERENCES job_order_workers(id);

UPDATE job_order_items it SET designer_id = w.id
    FROM job_order_workers w
    WHERE COALESCE(it.designer_name,'') <> '' AND TRIM(it.designer_name) = w.name;

UPDATE job_order_items it SET cutter_id = w.id
    FROM job_order_workers w
    WHERE COALESCE(it.cutter_name,'') <> '' AND TRIM(it.cutter_name) = w.name;

UPDATE job_order_items it SET stitcher_id = w.id
    FROM job_order_workers w
    WHERE COALESCE(it.stitcher_name,'') <> '' AND TRIM(it.stitcher_name) = w.name;

UPDATE job_order_items it SET hand_worker_id = w.id
    FROM job_order_workers w
    WHERE COALESCE(it.hand_worker_name,'') <> '' AND TRIM(it.hand_worker_name) = w.name;

ALTER TABLE job_order_items
    DROP COLUMN IF EXISTS designer_name,
    DROP COLUMN IF EXISTS cutter_name,
    DROP COLUMN IF EXISTS stitcher_name,
    DROP COLUMN IF EXISTS hand_worker_name;

CREATE INDEX IF NOT EXISTS idx_job_order_items_designer_id ON job_order_items(designer_id);
CREATE INDEX IF NOT EXISTS idx_job_order_items_cutter_id ON job_order_items(cutter_id);
CREATE INDEX IF NOT EXISTS idx_job_order_items_stitcher_id ON job_order_items(stitcher_id);
CREATE INDEX IF NOT EXISTS idx_job_order_items_hand_worker_id ON job_order_items(hand_worker_id);

-- job_order_item_work_logs: same treatment for worker_name -> worker_id
ALTER TABLE job_order_item_work_logs
    ADD COLUMN IF NOT EXISTS worker_id UUID REFERENCES job_order_workers(id);

UPDATE job_order_item_work_logs wl SET worker_id = w.id
    FROM job_order_workers w
    WHERE COALESCE(wl.worker_name,'') <> '' AND TRIM(wl.worker_name) = w.name;

ALTER TABLE job_order_item_work_logs
    DROP COLUMN IF EXISTS worker_name;

CREATE INDEX IF NOT EXISTS idx_job_order_item_work_logs_worker_id ON job_order_item_work_logs(worker_id);

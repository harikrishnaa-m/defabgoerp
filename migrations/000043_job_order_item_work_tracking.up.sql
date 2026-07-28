-- Add hand worker name to job order items, plus per-role work time logs
-- (designer / cutter / stitcher / hand worker) with start & end timestamps.

ALTER TABLE job_order_items
    ADD COLUMN IF NOT EXISTS hand_worker_name VARCHAR(100) DEFAULT '';

CREATE TABLE IF NOT EXISTS job_order_item_work_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_order_item_id UUID NOT NULL REFERENCES job_order_items(id) ON DELETE CASCADE,
    role              VARCHAR(20) NOT NULL,  -- DESIGNER, CUTTER, STITCHER, HAND_WORKER
    worker_name       VARCHAR(100) DEFAULT '',
    started_at        TIMESTAMPTZ NOT NULL,
    ended_at          TIMESTAMPTZ,
    notes             TEXT DEFAULT '',
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_order_item_work_logs_item ON job_order_item_work_logs(job_order_item_id);
CREATE INDEX IF NOT EXISTS idx_job_order_item_work_logs_role ON job_order_item_work_logs(role);

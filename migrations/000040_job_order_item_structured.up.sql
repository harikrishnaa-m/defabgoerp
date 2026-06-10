-- Restructure job_order_items: replace flat description with
-- structured category / sub_category / pieces (JSONB array)
-- Each piece has: piece_type, with_lining, works[]
-- Single-piece garments have one entry in pieces with piece_type = ''

ALTER TABLE job_order_items
    ADD COLUMN IF NOT EXISTS category     VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sub_category VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pieces       JSONB        NOT NULL DEFAULT '[]';

-- Keep description column for backwards compatibility (nullable going forward)
ALTER TABLE job_order_items
    ALTER COLUMN description DROP NOT NULL;

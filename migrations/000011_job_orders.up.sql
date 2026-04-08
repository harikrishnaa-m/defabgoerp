-- Job Orders module: dyeing, stitching, alteration tracking with self-contained billing

CREATE TABLE IF NOT EXISTS job_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_number      VARCHAR(20) NOT NULL UNIQUE,
    customer_id     UUID NOT NULL REFERENCES customers(id),
    branch_id       UUID REFERENCES branches(id),
    warehouse_id    UUID REFERENCES warehouses(id),
    job_type        VARCHAR(50) NOT NULL,          -- DYEING, STITCHING, ALTERATION, etc.
    material_source VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER',  -- CUSTOMER or STORE
    status          VARCHAR(50) NOT NULL DEFAULT 'RECEIVED',
    payment_status  VARCHAR(20) NOT NULL DEFAULT 'UNPAID',    -- UNPAID, PARTIAL, PAID
    received_date   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expected_delivery_date DATE,
    actual_delivery_date   TIMESTAMPTZ,
    sub_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    gst_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    net_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    notes           TEXT DEFAULT '',
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS job_order_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_order_id    UUID NOT NULL REFERENCES job_orders(id) ON DELETE CASCADE,
    description     TEXT NOT NULL,
    quantity        DECIMAL(10,2) NOT NULL DEFAULT 1,
    unit_price      DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount        DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_percent     DECIMAL(5,2) NOT NULL DEFAULT 0,
    cgst            DECIMAL(12,2) NOT NULL DEFAULT 0,
    sgst            DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_price     DECIMAL(12,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS job_order_materials (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_order_id          UUID NOT NULL REFERENCES job_orders(id) ON DELETE CASCADE,
    raw_material_stock_id UUID NOT NULL REFERENCES raw_material_stocks(id),
    quantity_used         DECIMAL(10,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS job_order_status_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_order_id    UUID NOT NULL REFERENCES job_orders(id) ON DELETE CASCADE,
    status          VARCHAR(50) NOT NULL,
    notes           TEXT DEFAULT '',
    updated_by      UUID NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS job_order_payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_order_id    UUID NOT NULL REFERENCES job_orders(id) ON DELETE CASCADE,
    amount          DECIMAL(12,2) NOT NULL,
    payment_method  VARCHAR(30) NOT NULL,  -- CASH, CARD, UPI, BANK_TRANSFER
    reference       VARCHAR(100) DEFAULT '',
    paid_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_orders_customer ON job_orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_job_orders_branch ON job_orders(branch_id);
CREATE INDEX IF NOT EXISTS idx_job_orders_status ON job_orders(status);
CREATE INDEX IF NOT EXISTS idx_job_orders_job_type ON job_orders(job_type);
CREATE INDEX IF NOT EXISTS idx_job_order_items_order ON job_order_items(job_order_id);
CREATE INDEX IF NOT EXISTS idx_job_order_materials_order ON job_order_materials(job_order_id);
CREATE INDEX IF NOT EXISTS idx_job_order_status_history_order ON job_order_status_history(job_order_id);
CREATE INDEX IF NOT EXISTS idx_job_order_payments_order ON job_order_payments(job_order_id);

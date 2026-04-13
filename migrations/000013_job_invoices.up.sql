-- Job Invoices: auto-created when a job order is created

CREATE TABLE IF NOT EXISTS job_invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_number  VARCHAR(20) NOT NULL UNIQUE,
    job_order_id    UUID NOT NULL REFERENCES job_orders(id) ON DELETE CASCADE,
    branch_id       UUID REFERENCES branches(id),
    customer_id     UUID NOT NULL REFERENCES customers(id),
    sub_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    gst_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    net_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    payment_status  VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_job_invoices_job_order ON job_invoices(job_order_id);
CREATE INDEX IF NOT EXISTS idx_job_invoices_branch ON job_invoices(branch_id);
CREATE INDEX IF NOT EXISTS idx_job_invoices_invoice_number ON job_invoices(invoice_number);

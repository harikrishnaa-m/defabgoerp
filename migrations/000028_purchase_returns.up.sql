CREATE TABLE IF NOT EXISTS purchase_returns (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pr_number           VARCHAR(50) UNIQUE NOT NULL,
    pr_date             DATE NOT NULL,
    supplier_id         UUID NOT NULL REFERENCES suppliers(id),
    purchase_invoice_id UUID REFERENCES purchase_invoices(id),
    goods_receipt_id    UUID REFERENCES goods_receipts(id),
    currency            VARCHAR(10) NOT NULL DEFAULT 'Rs',
    exchange_rate       NUMERIC NOT NULL DEFAULT 1,
    sub_amount          NUMERIC NOT NULL DEFAULT 0,
    tax_amount          NUMERIC NOT NULL DEFAULT 0,
    duty_amount         NUMERIC NOT NULL DEFAULT 0,
    round_off           NUMERIC NOT NULL DEFAULT 0,
    net_amount          NUMERIC NOT NULL DEFAULT 0,
    reason              TEXT,
    status              VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS purchase_return_items (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_return_id      UUID NOT NULL REFERENCES purchase_returns(id) ON DELETE CASCADE,
    purchase_order_item_id  UUID REFERENCES purchase_order_items(id),
    item_name               VARCHAR(200) NOT NULL,
    hsn_code                VARCHAR(50),
    unit                    VARCHAR(50),
    quantity                NUMERIC NOT NULL,
    unit_price              NUMERIC NOT NULL DEFAULT 0,
    gst_percent             NUMERIC NOT NULL DEFAULT 0,
    gst_amount              NUMERIC NOT NULL DEFAULT 0,
    total_amount            NUMERIC NOT NULL DEFAULT 0,
    reason                  VARCHAR(200),
    tax_inclusive           BOOLEAN NOT NULL DEFAULT FALSE
);

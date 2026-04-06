-- Create returns tables
CREATE TABLE return_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    return_number VARCHAR(50) NOT NULL UNIQUE,
    sales_invoice_id UUID NOT NULL REFERENCES sales_invoices(id) ON DELETE CASCADE,
    branch_id UUID REFERENCES branches(id),
    warehouse_id UUID REFERENCES warehouses(id),
    customer_id UUID REFERENCES customers(id),
    status VARCHAR(30) NOT NULL DEFAULT 'REQUESTED',
    refund_type VARCHAR(20) NOT NULL DEFAULT 'CASH',
    refund_method VARCHAR(20),
    refund_reference VARCHAR(100),
    refund_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    gst_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    notes TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE return_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    return_order_id UUID NOT NULL REFERENCES return_orders(id) ON DELETE CASCADE,
    sales_invoice_item_id UUID NOT NULL REFERENCES sales_invoice_items(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES variants(id),
    quantity INT NOT NULL,
    unit_price DECIMAL(12,2) NOT NULL,
    discount DECIMAL(12,2) NOT NULL DEFAULT 0,
    bill_discount_share DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_price DECIMAL(12,2) NOT NULL DEFAULT 0,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE return_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    return_order_id UUID NOT NULL REFERENCES return_orders(id) ON DELETE CASCADE,
    amount DECIMAL(12,2) NOT NULL,
    payment_method VARCHAR(20) NOT NULL,
    reference VARCHAR(100),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_return_orders_sales_invoice_id ON return_orders(sales_invoice_id);
CREATE INDEX idx_return_orders_branch_id ON return_orders(branch_id);
CREATE INDEX idx_return_orders_status ON return_orders(status);
CREATE INDEX idx_return_items_return_order_id ON return_items(return_order_id);
CREATE INDEX idx_return_items_sales_invoice_item_id ON return_items(sales_invoice_item_id);
CREATE INDEX idx_return_payments_return_order_id ON return_payments(return_order_id);

-- ============================================================
-- Sales Module Tables
-- ============================================================

-- 1. Customers
CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_code VARCHAR(50) UNIQUE,
    name VARCHAR(200) NOT NULL,
    phone VARCHAR(30),
    email VARCHAR(150),
    total_purchases DECIMAL(14,2) NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Sales Persons
CREATE TABLE sales_persons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    branch_id UUID REFERENCES branches(id),
    name VARCHAR(200) NOT NULL,
    employee_code VARCHAR(50) UNIQUE,
    phone VARCHAR(30),
    email VARCHAR(150),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sales_persons_user_id ON sales_persons(user_id);
CREATE INDEX idx_sales_persons_branch_id ON sales_persons(branch_id);

-- 3. Sales Orders
CREATE TABLE sales_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    so_number VARCHAR(50) NOT NULL UNIQUE,
    channel VARCHAR(20) NOT NULL DEFAULT 'STORE',
    branch_id UUID REFERENCES branches(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    salesperson_id UUID REFERENCES sales_persons(id),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    created_by UUID NOT NULL REFERENCES users(id),
    order_date TIMESTAMPTZ NOT NULL,
    subtotal DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_total DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_total DECIMAL(12,2) NOT NULL DEFAULT 0,
    grand_total DECIMAL(12,2) NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'DRAFT',
    payment_status VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sales_orders_branch_id ON sales_orders(branch_id);
CREATE INDEX idx_sales_orders_customer_id ON sales_orders(customer_id);
CREATE INDEX idx_sales_orders_salesperson_id ON sales_orders(salesperson_id);
CREATE INDEX idx_sales_orders_warehouse_id ON sales_orders(warehouse_id);
CREATE INDEX idx_sales_orders_status ON sales_orders(status);

-- 4. Sales Order Items
CREATE TABLE sales_order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sales_order_id UUID NOT NULL REFERENCES sales_orders(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES variants(id),
    quantity INT NOT NULL,
    unit_price DECIMAL(12,2) NOT NULL,
    discount DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_price DECIMAL(12,2) NOT NULL DEFAULT 0
);
CREATE INDEX idx_sales_order_items_sales_order_id ON sales_order_items(sales_order_id);
CREATE INDEX idx_sales_order_items_variant_id ON sales_order_items(variant_id);

-- 5. Sales Invoices
CREATE TABLE sales_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sales_order_id UUID NOT NULL UNIQUE REFERENCES sales_orders(id),
    customer_id UUID NOT NULL REFERENCES customers(id),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    channel VARCHAR(20) NOT NULL DEFAULT 'STORE',
    branch_id UUID REFERENCES branches(id),
    invoice_number VARCHAR(50) NOT NULL UNIQUE,
    invoice_date TIMESTAMPTZ NOT NULL,
    sub_amount DECIMAL(12,2) DEFAULT 0,
    discount_amount DECIMAL(12,2) DEFAULT 0,
    gst_amount DECIMAL(12,2) DEFAULT 0,
    round_off DECIMAL(12,2) DEFAULT 0,
    net_amount DECIMAL(12,2) NOT NULL,
    paid_amount DECIMAL(12,2) DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sales_invoices_customer_id ON sales_invoices(customer_id);
CREATE INDEX idx_sales_invoices_warehouse_id ON sales_invoices(warehouse_id);
CREATE INDEX idx_sales_invoices_branch_id ON sales_invoices(branch_id);
CREATE INDEX idx_sales_invoices_invoice_date ON sales_invoices(invoice_date);
CREATE INDEX idx_sales_invoices_status ON sales_invoices(status);

-- 6. Sales Invoice Items
CREATE TABLE sales_invoice_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sales_invoice_id UUID NOT NULL REFERENCES sales_invoices(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES variants(id),
    quantity INT NOT NULL,
    unit_price DECIMAL(12,2) NOT NULL,
    discount DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_price DECIMAL(12,2) NOT NULL DEFAULT 0
);
CREATE INDEX idx_sales_invoice_items_sales_invoice_id ON sales_invoice_items(sales_invoice_id);
CREATE INDEX idx_sales_invoice_items_variant_id ON sales_invoice_items(variant_id);

-- 7. Sales Payments
CREATE TABLE sales_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sales_invoice_id UUID NOT NULL REFERENCES sales_invoices(id) ON DELETE CASCADE,
    amount DECIMAL(12,2) NOT NULL,
    payment_method VARCHAR(20) NOT NULL,
    reference VARCHAR(50),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sales_payments_sales_invoice_id ON sales_payments(sales_invoice_id);
CREATE INDEX idx_sales_payments_payment_method ON sales_payments(payment_method);

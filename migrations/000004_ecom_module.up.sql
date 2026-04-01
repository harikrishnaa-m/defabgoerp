-- ============================================================
-- E-Commerce Module Tables
-- ============================================================

-- 1. E-Commerce Customers (separate from ERP walk-in customers)
CREATE TABLE ecom_customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    phone VARCHAR(30),
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Customer Addresses
CREATE TABLE ecom_addresses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID NOT NULL REFERENCES ecom_customers(id) ON DELETE CASCADE,
    label VARCHAR(50) DEFAULT 'Home',       -- Home, Work, Other
    full_name VARCHAR(200) NOT NULL,
    phone VARCHAR(30) NOT NULL,
    address_line1 VARCHAR(300) NOT NULL,
    address_line2 VARCHAR(300),
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100) NOT NULL,
    pincode VARCHAR(10) NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_ecom_addresses_customer ON ecom_addresses(customer_id);

-- 3. Shopping Cart (one active cart per customer)
CREATE TABLE ecom_carts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID UNIQUE NOT NULL REFERENCES ecom_customers(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE ecom_cart_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cart_id UUID NOT NULL REFERENCES ecom_carts(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES variants(id),
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(cart_id, variant_id)
);
CREATE INDEX idx_ecom_cart_items_cart ON ecom_cart_items(cart_id);

-- 4. Orders
CREATE TABLE ecom_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number VARCHAR(50) UNIQUE NOT NULL,
    customer_id UUID NOT NULL REFERENCES ecom_customers(id),
    address_id UUID REFERENCES ecom_addresses(id),

    -- Snapshot of address at order time
    shipping_name VARCHAR(200),
    shipping_phone VARCHAR(30),
    shipping_address TEXT,
    shipping_city VARCHAR(100),
    shipping_state VARCHAR(100),
    shipping_pincode VARCHAR(10),

    item_count INT NOT NULL DEFAULT 0,
    sub_total DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    shipping_charge DECIMAL(10,2) NOT NULL DEFAULT 0,
    grand_total DECIMAL(12,2) NOT NULL DEFAULT 0,

    status VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    -- PENDING, CONFIRMED, PROCESSING, SHIPPED, DELIVERED, CANCELLED, RETURNED

    payment_status VARCHAR(30) NOT NULL DEFAULT 'UNPAID',
    -- UNPAID, PAID, REFUNDED

    payment_method VARCHAR(50),  -- COD, RAZORPAY, UPI, etc.
    payment_ref VARCHAR(200),    -- external payment gateway reference

    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_ecom_orders_customer ON ecom_orders(customer_id);
CREATE INDEX idx_ecom_orders_status ON ecom_orders(status);

CREATE TABLE ecom_order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES ecom_orders(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES variants(id),
    product_name VARCHAR(200) NOT NULL,
    variant_name VARCHAR(150) NOT NULL,
    sku VARCHAR(100) NOT NULL,
    variant_code INT NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(12,2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_ecom_order_items_order ON ecom_order_items(order_id);

-- Sequence for order numbers: ECOM-00001, ECOM-00002 ...
CREATE SEQUENCE ecom_order_seq START 1;

-- Add Google OAuth support to ecom_customers
ALTER TABLE ecom_customers
    ADD COLUMN google_id VARCHAR(255) UNIQUE,
    ALTER COLUMN password_hash DROP NOT NULL;

-- Add password reset support to ecom_customers
ALTER TABLE ecom_customers
    ADD COLUMN reset_token VARCHAR(255),
    ADD COLUMN reset_token_expiry TIMESTAMPTZ;

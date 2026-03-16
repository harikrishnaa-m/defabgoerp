-- 1. Enable UUID Extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 2. Auth & RBAC
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL, -- Admin, StoreManager, StitchingManager
    permissions TEXT -- JSON blob
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role_id INT REFERENCES roles(id),
    branch_id INT, -- Nullable (Admin has no branch)
    refresh_token VARCHAR(255), -- For storing refresh token securely
    reset_token VARCHAR(255), -- For password reset functionality
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 3. Locations (Store & Factory)
CREATE TABLE branches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    address TEXT,
    manager_id UUID REFERENCES users(id)
);
ALTER TABLE branches ADD COLUMN created_at TIMESTAMPTZ DEFAULT NOW();

-- Add new columns to branches table
ALTER TABLE branches ADD COLUMN branch_code VARCHAR(50);
ALTER TABLE branches ADD COLUMN city VARCHAR(100);
ALTER TABLE branches ADD COLUMN state VARCHAR(100);
ALTER TABLE branches ADD COLUMN phone_number VARCHAR(20);

CREATE TABLE warehouses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id UUID REFERENCES branches(id),
    name VARCHAR(150) NOT NULL,
    type VARCHAR(20) DEFAULT 'STORE' -- CENTRAL, STORE, FACTORY
);
ALTER TABLE warehouses ADD COLUMN created_at TIMESTAMPTZ DEFAULT NOW();

-- Add warehouse_code column
ALTER TABLE warehouses ADD COLUMN warehouse_code VARCHAR(50);



-- 4. Product Catalog (The "Meters" Logic)
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL,
    products_count INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    fabric_composition VARCHAR(200),
    pattern VARCHAR(100),
    occasion VARCHAR(100),
    care_instructions VARCHAR(200),
    category_id UUID REFERENCES categories(id),
    brand VARCHAR(100),
    main_image_url VARCHAR(500), -- Main image for product
    is_web_visible BOOLEAN DEFAULT TRUE,
    is_stitched BOOLEAN DEFAULT FALSE,
    uom VARCHAR(20) DEFAULT 'Unit',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
);

    -- Attribute tables
 CREATE TABLE attributes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

    CREATE TABLE attribute_values (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        attribute_id UUID REFERENCES attributes(id) ON DELETE CASCADE,
        value VARCHAR(200) NOT NULL,
        is_active BOOLEAN DEFAULT TRUE,
        created_at TIMESTAMPTZ DEFAULT NOW()
    );

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES products(id),
    image_url VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE variants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id UUID REFERENCES products(id),
    name VARCHAR(150) NOT NULL,
    sku VARCHAR(100) UNIQUE NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    cost_price DECIMAL(10, 2),
    barcode VARCHAR(100) UNIQUE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE variant_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variant_id UUID REFERENCES variants(id),
    image_url VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE barcodes (
    id SERIAL PRIMARY KEY,
    variant_id UUID REFERENCES variants(id),
    code VARCHAR(100) UNIQUE NOT NULL,
    generated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Variant Attribute Mapping
CREATE TABLE variant_attribute_mapping (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variant_id UUID REFERENCES variants(id) ON DELETE CASCADE,
    attribute_value_id UUID REFERENCES attribute_values(id) ON DELETE CASCADE
);

CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    supplier_code VARCHAR(50) UNIQUE,
    name VARCHAR(200) NOT NULL,
    phone VARCHAR(30),
    email VARCHAR(150),
    address TEXT,
    gst_number VARCHAR(15) UNIQUE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE purchase_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    po_number VARCHAR(50) NOT NULL,
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    status VARCHAR(30) DEFAULT 'DRAFT',
    order_date TIMESTAMPTZ,
    expected_date TIMESTAMPTZ,
    total_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    grand_total DECIMAL(12,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE purchase_order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    item_name VARCHAR(200) NOT NULL,
    description TEXT,
    hsn_code VARCHAR(20),
    unit VARCHAR(20) NOT NULL,
    quantity DECIMAL(10,2) NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    gst_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    gst_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    total_price DECIMAL(12,2) NOT NULL,
    received_qty DECIMAL(10,2) DEFAULT 0
);

CREATE TABLE warehouse_stocks (
    id SERIAL PRIMARY KEY,
    warehouse_id INT REFERENCES warehouses(id),
    variant_id UUID REFERENCES variants(id),
    quantity DECIMAL(10, 2) DEFAULT 0,    -- Supports 1.50 meters
    reserved DECIMAL(10, 2) DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(warehouse_id, variant_id)      -- Prevents duplicate rows
);

-- Stock table (new, aligns with Go model)
CREATE TABLE stocks (
     id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
     variant_id UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
     warehouse_id UUID NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
     quantity DECIMAL(10, 2) NOT NULL DEFAULT 0,
     updated_at TIMESTAMPTZ DEFAULT NOW(),
     UNIQUE(variant_id, warehouse_id)
);

-- Add stock_type column
ALTER TABLE stocks ADD COLUMN stock_type VARCHAR(20) NOT NULL DEFAULT 'PRODUCT';

CREATE TABLE stock_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    variant_id UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    from_warehouse_id UUID REFERENCES warehouses(id),
    to_warehouse_id UUID REFERENCES warehouses(id),
    quantity DECIMAL(10, 2) NOT NULL,
    movement_type VARCHAR(20) NOT NULL,          -- IN, OUT, TRANSFER
    stock_request_id UUID,
    status VARCHAR(20) DEFAULT 'COMPLETED',      -- PENDING, IN_TRANSIT, RECEIVED, CANCELLED, COMPLETED
    purchase_order_id UUID,
    supplier_id UUID,
    reference VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS sale_order_id UUID;
-- 6. Billing & Finance (Split Payments)
CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_number VARCHAR(50) UNIQUE NOT NULL,
    branch_id INT REFERENCES branches(id),
    total_amount DECIMAL(10, 2) NOT NULL,
    tax_amount DECIMAL(10, 2) DEFAULT 0,
    type VARCHAR(20) DEFAULT 'POS',       -- POS or WEB
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE payments (
    id SERIAL PRIMARY KEY,
    invoice_id UUID REFERENCES invoices(id),
    method VARCHAR(50),                   -- Cash, Card, UPI, StoreCredit
    amount DECIMAL(10, 2) NOT NULL,
    reference VARCHAR(100)                -- Trans ID
);

-- Goods Receipts (GRN)
CREATE TABLE goods_receipts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    grn_number VARCHAR(50) UNIQUE NOT NULL,
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    received_by UUID NOT NULL,
    received_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reference VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE goods_receipt_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    goods_receipt_id UUID NOT NULL REFERENCES goods_receipts(id) ON DELETE CASCADE,
    purchase_order_item_id UUID NOT NULL REFERENCES purchase_order_items(id),
    ordered_qty DECIMAL(10,2) NOT NULL,
    received_qty DECIMAL(10,2) NOT NULL
);

-- Raw Material Stock (tracks raw material inventory per warehouse)
CREATE TABLE raw_material_stocks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_name VARCHAR(200) NOT NULL,
    hsn_code VARCHAR(20),
    unit VARCHAR(20) NOT NULL,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    quantity DECIMAL(10,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(item_name, warehouse_id)
);

-- Raw Material Stock Movements (audit trail for raw material stock changes)
CREATE TABLE raw_material_movements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_name VARCHAR(200) NOT NULL,
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    quantity DECIMAL(10,2) NOT NULL,
    movement_type VARCHAR(20) NOT NULL, -- IN, OUT, ADJUSTMENT
    goods_receipt_id UUID REFERENCES goods_receipts(id),
    purchase_order_id UUID REFERENCES purchase_orders(id),
    reference VARCHAR(200),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Purchase Invoices (supplier bills linked to PO)
CREATE TABLE purchase_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_number VARCHAR(50) UNIQUE NOT NULL,
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    warehouse_id UUID NOT NULL REFERENCES warehouses(id),
    invoice_date DATE NOT NULL,
    sub_amount DECIMAL(12,2) DEFAULT 0,
    discount_amount DECIMAL(12,2) DEFAULT 0,
    gst_amount DECIMAL(12,2) DEFAULT 0,
    round_off DECIMAL(12,2) DEFAULT 0,
    net_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    paid_amount DECIMAL(12,2) DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    notes TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Purchase Invoice Items
CREATE TABLE purchase_invoice_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_invoice_id UUID NOT NULL REFERENCES purchase_invoices(id) ON DELETE CASCADE,
    purchase_order_item_id UUID NOT NULL REFERENCES purchase_order_items(id),
    item_name VARCHAR(200) NOT NULL,
    hsn_code VARCHAR(20),
    unit VARCHAR(30),
    quantity DECIMAL(10,3) NOT NULL,
    unit_price DECIMAL(12,2) NOT NULL,
    tax_percent DECIMAL(5,2) DEFAULT 0,
    tax_amount DECIMAL(12,2) DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL
);

-- Supplier Payments
CREATE TABLE supplier_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_invoice_id UUID NOT NULL REFERENCES purchase_invoices(id) ON DELETE CASCADE,
    amount DECIMAL(12,2) NOT NULL,
    payment_method VARCHAR(20) NOT NULL,
    reference VARCHAR(100),
    paid_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
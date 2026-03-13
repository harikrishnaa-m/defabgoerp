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
    id SERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    contact VARCHAR(100),
    email VARCHAR(150)
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

CREATE TABLE stock_movements (
    id SERIAL PRIMARY KEY,
    variant_id UUID REFERENCES variants(id),
    from_warehouse_id INT REFERENCES warehouses(id),
    to_warehouse_id INT REFERENCES warehouses(id),
    quantity DECIMAL(10, 2),
    type VARCHAR(50),                     -- PURCHASE, SALE, TRANSFER, MANUFACTURING
    reference_id VARCHAR(100),            -- Invoice ID or PO Number
    created_at TIMESTAMPTZ DEFAULT NOW()
);

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
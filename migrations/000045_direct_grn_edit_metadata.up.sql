ALTER TABLE purchase_order_items
    ADD COLUMN IF NOT EXISTS category_id UUID REFERENCES categories(id),
    ADD COLUMN IF NOT EXISTS product_id UUID REFERENCES products(id),
    ADD COLUMN IF NOT EXISTS create_variant BOOLEAN NOT NULL DEFAULT FALSE;

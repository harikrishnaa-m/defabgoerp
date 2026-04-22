-- Allow sales_invoice_items and sales_order_items to be created without a linked variant (for migrated / POS data)
ALTER TABLE sales_invoice_items ALTER COLUMN variant_id DROP NOT NULL;
ALTER TABLE sales_invoice_items ADD COLUMN IF NOT EXISTS item_description TEXT;

ALTER TABLE sales_order_items ALTER COLUMN variant_id DROP NOT NULL;
ALTER TABLE sales_order_items ADD COLUMN IF NOT EXISTS item_description TEXT;

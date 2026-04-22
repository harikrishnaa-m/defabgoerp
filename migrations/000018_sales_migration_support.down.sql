ALTER TABLE sales_invoice_items DROP COLUMN IF EXISTS item_description;
ALTER TABLE sales_invoice_items ALTER COLUMN variant_id SET NOT NULL;

ALTER TABLE sales_order_items DROP COLUMN IF EXISTS item_description;
ALTER TABLE sales_order_items ALTER COLUMN variant_id SET NOT NULL;

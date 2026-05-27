ALTER TABLE sales_invoices DROP COLUMN IF EXISTS return_order_id;
ALTER TABLE sales_orders  DROP COLUMN IF EXISTS return_order_id;

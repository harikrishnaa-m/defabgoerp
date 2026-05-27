ALTER TABLE sales_orders  ADD COLUMN IF NOT EXISTS return_order_id UUID REFERENCES return_orders(id);
ALTER TABLE sales_invoices ADD COLUMN IF NOT EXISTS return_order_id UUID REFERENCES return_orders(id);

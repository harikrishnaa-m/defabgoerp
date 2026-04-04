ALTER TABLE sales_order_items ALTER COLUMN quantity TYPE INT USING quantity::INT;
ALTER TABLE sales_invoice_items ALTER COLUMN quantity TYPE INT USING quantity::INT;

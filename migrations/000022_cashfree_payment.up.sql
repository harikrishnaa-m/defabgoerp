-- Add Cashfree order tracking fields to ecom_orders
ALTER TABLE ecom_orders
    ADD COLUMN IF NOT EXISTS cf_order_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS payment_session_id VARCHAR(500);

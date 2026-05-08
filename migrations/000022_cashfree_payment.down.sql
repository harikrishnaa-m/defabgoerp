ALTER TABLE ecom_orders
    DROP COLUMN IF EXISTS cf_order_id,
    DROP COLUMN IF EXISTS payment_session_id;

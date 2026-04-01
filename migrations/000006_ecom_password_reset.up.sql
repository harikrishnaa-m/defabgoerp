-- Add password reset support to ecom_customers
ALTER TABLE ecom_customers
    ADD COLUMN reset_token VARCHAR(255),
    ADD COLUMN reset_token_expiry TIMESTAMPTZ;

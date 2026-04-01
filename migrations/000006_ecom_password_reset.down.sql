-- Revert password reset support
ALTER TABLE ecom_customers
    DROP COLUMN IF EXISTS reset_token,
    DROP COLUMN IF EXISTS reset_token_expiry;

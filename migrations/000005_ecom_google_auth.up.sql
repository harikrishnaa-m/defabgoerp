-- Add Google OAuth support to ecom_customers
ALTER TABLE ecom_customers
    ADD COLUMN google_id VARCHAR(255) UNIQUE,
    ALTER COLUMN password_hash DROP NOT NULL;

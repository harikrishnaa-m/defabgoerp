-- Revert Google OAuth support
ALTER TABLE ecom_customers
    DROP COLUMN IF EXISTS google_id,
    ALTER COLUMN password_hash SET NOT NULL;

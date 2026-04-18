ALTER TABLE job_orders
    DROP COLUMN IF EXISTS sample_provided,
    DROP COLUMN IF EXISTS sample_description,
    DROP COLUMN IF EXISTS measurement_bill_number;

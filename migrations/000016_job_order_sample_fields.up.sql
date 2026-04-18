ALTER TABLE job_orders
    ADD COLUMN sample_provided BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN sample_description TEXT NOT NULL DEFAULT '',
    ADD COLUMN measurement_bill_number VARCHAR(100) NOT NULL DEFAULT '';

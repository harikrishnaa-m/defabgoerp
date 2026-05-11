-- Revert to unconstrained NUMERIC
ALTER TABLE purchase_returns
    ALTER COLUMN exchange_rate TYPE NUMERIC,
    ALTER COLUMN sub_amount    TYPE NUMERIC,
    ALTER COLUMN tax_amount    TYPE NUMERIC,
    ALTER COLUMN duty_amount   TYPE NUMERIC,
    ALTER COLUMN round_off     TYPE NUMERIC,
    ALTER COLUMN net_amount    TYPE NUMERIC;

ALTER TABLE purchase_return_items
    ALTER COLUMN quantity      TYPE NUMERIC,
    ALTER COLUMN unit_price    TYPE NUMERIC,
    ALTER COLUMN gst_percent   TYPE NUMERIC,
    ALTER COLUMN gst_amount    TYPE NUMERIC,
    ALTER COLUMN total_amount  TYPE NUMERIC;

-- Enforce 2-decimal precision on purchase_returns monetary columns
ALTER TABLE purchase_returns
    ALTER COLUMN exchange_rate TYPE NUMERIC(12,2),
    ALTER COLUMN sub_amount    TYPE NUMERIC(12,2),
    ALTER COLUMN tax_amount    TYPE NUMERIC(12,2),
    ALTER COLUMN duty_amount   TYPE NUMERIC(12,2),
    ALTER COLUMN round_off     TYPE NUMERIC(12,2),
    ALTER COLUMN net_amount    TYPE NUMERIC(12,2);

-- Enforce 2-decimal precision on purchase_return_items monetary columns
ALTER TABLE purchase_return_items
    ALTER COLUMN quantity      TYPE NUMERIC(12,4),
    ALTER COLUMN unit_price    TYPE NUMERIC(12,2),
    ALTER COLUMN gst_percent   TYPE NUMERIC(5,2),
    ALTER COLUMN gst_amount    TYPE NUMERIC(12,2),
    ALTER COLUMN total_amount  TYPE NUMERIC(12,2);

DROP TABLE IF EXISTS exchange_settlements;
DROP TABLE IF EXISTS exchange_items_in;
DROP TABLE IF EXISTS exchange_items_out;
DROP TABLE IF EXISTS exchange_orders;
ALTER TABLE return_orders DROP COLUMN IF EXISTS source;

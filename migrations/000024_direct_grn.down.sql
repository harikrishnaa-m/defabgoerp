DROP TABLE IF EXISTS purchase_charges;

ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS credit_amount;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS cash_amount;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_to;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS paid_by;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS additional_work_amount;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS additional_work;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS free_qty;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS category;
ALTER TABLE purchase_order_items DROP COLUMN IF EXISTS product_code;

ALTER TABLE goods_receipts DROP COLUMN IF EXISTS lr_number;
ALTER TABLE goods_receipts DROP COLUMN IF EXISTS transport;

ALTER TABLE purchase_orders DROP COLUMN IF EXISTS purchase_type;

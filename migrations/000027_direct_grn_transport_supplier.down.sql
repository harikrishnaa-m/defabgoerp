ALTER TABLE goods_receipts DROP COLUMN IF EXISTS transport_supplier_id;

ALTER TABLE goods_receipts
    ADD COLUMN IF NOT EXISTS transport VARCHAR(100);

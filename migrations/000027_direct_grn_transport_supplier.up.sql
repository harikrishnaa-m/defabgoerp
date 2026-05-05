-- Change transport from VARCHAR to UUID FK referencing suppliers
ALTER TABLE goods_receipts DROP COLUMN IF EXISTS transport;

ALTER TABLE goods_receipts
    ADD COLUMN IF NOT EXISTS transport_supplier_id UUID REFERENCES suppliers(id) ON DELETE SET NULL;

-- Production Orders: raw material → finished product conversion

CREATE TABLE IF NOT EXISTS production_orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_number VARCHAR(20) NOT NULL UNIQUE,
    branch_id         UUID REFERENCES branches(id),
    warehouse_id      UUID NOT NULL REFERENCES warehouses(id),
    output_variant_id UUID NOT NULL REFERENCES variants(id),
    output_quantity   DECIMAL(10,2) NOT NULL DEFAULT 1,
    status            VARCHAR(50) NOT NULL DEFAULT 'PLANNED',
    notes             TEXT DEFAULT '',
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    created_by        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS production_materials (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_order_id   UUID NOT NULL REFERENCES production_orders(id) ON DELETE CASCADE,
    raw_material_stock_id UUID NOT NULL REFERENCES raw_material_stocks(id),
    quantity_used         DECIMAL(10,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS production_status_history (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_order_id UUID NOT NULL REFERENCES production_orders(id) ON DELETE CASCADE,
    status              VARCHAR(50) NOT NULL,
    notes               TEXT DEFAULT '',
    updated_by          UUID NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_production_orders_branch ON production_orders(branch_id);
CREATE INDEX IF NOT EXISTS idx_production_orders_status ON production_orders(status);
CREATE INDEX IF NOT EXISTS idx_production_orders_output ON production_orders(output_variant_id);
CREATE INDEX IF NOT EXISTS idx_production_materials_order ON production_materials(production_order_id);
CREATE INDEX IF NOT EXISTS idx_production_status_history_order ON production_status_history(production_order_id);

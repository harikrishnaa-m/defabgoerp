package production

import "time"

type ProductionOrder struct {
	ID               string     `json:"id"`
	ProductionNumber string     `json:"production_number"`
	BranchID         *string    `json:"branch_id"`
	WarehouseID      string     `json:"warehouse_id"`
	OutputVariantID  string     `json:"output_variant_id"`
	OutputQuantity   float64    `json:"output_quantity"`
	Status           string     `json:"status"`
	Notes            string     `json:"notes"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ProductionMaterial struct {
	ID                 string  `json:"id"`
	ProductionOrderID  string  `json:"production_order_id"`
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

type ProductionStatusEntry struct {
	ID                string    `json:"id"`
	ProductionOrderID string    `json:"production_order_id"`
	Status            string    `json:"status"`
	Notes             string    `json:"notes"`
	UpdatedBy         string    `json:"updated_by"`
	UpdatedAt         time.Time `json:"updated_at"`
}

package rawmaterial

import "database/sql"

type RawMaterialStockRow struct {
	ID            string
	ItemName      string
	HSNCode       sql.NullString
	Unit          sql.NullString
	WarehouseID   string
	WarehouseName string
	Quantity      string
	UpdatedAt     string
}

type RawMaterialMovementRow struct {
	ID              string
	ItemName        string
	WarehouseID     string
	WarehouseName   string
	Quantity        string
	MovementType    string
	GoodsReceiptID  sql.NullString
	GRNNumber       sql.NullString
	PurchaseOrderID sql.NullString
	PONumber        sql.NullString
	Reference       sql.NullString
	CreatedAt       string
}

type AdjustStockInput struct {
	StockID   string  `json:"stock_id"`
	Quantity  float64 `json:"quantity"`
	Type      string  `json:"type"`      // OUT or ADJUSTMENT
	Reference string  `json:"reference"` // e.g. "Used for stitching batch #12"
}

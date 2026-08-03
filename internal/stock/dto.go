package stock

import "github.com/shopspring/decimal"

type StockCreateInput struct {
	VariantID   string          `json:"variant_id"`
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
	StockType   string          `json:"stock_type"` // RAW_MATERIAL or PRODUCT
}

type StockUpdateInput struct {
	VariantID   string          `json:"variant_id"`
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
	StockType   string          `json:"stock_type"`
}

type StockAdjustInput struct {
	NewQuantity decimal.Decimal `json:"new_quantity"`
	Reason      string          `json:"reason"`
}

// QuickAddInput creates a product (if needed), a variant, and stock in one shot.
type QuickAddInput struct {
	CategoryID string `json:"category_id"`

	// Supply one of these:
	ProductID   string `json:"product_id"`   // existing product
	ProductName string `json:"product_name"` // new product to create

	// Variant (variant_code is required)
	VariantCode int     `json:"variant_code"`
	VariantName string  `json:"variant_name"`
	Price       float64 `json:"price"`
	CostPrice   float64 `json:"cost_price"`
	HSNCode     string  `json:"hsn_code"`
	UOM         string  `json:"uom"`         // unit of measure for the product; defaults to "Unit" if omitted
	Description string  `json:"description"` // item/product description; only applied when a new product is created

	// Stock (optional) — either a single warehouse (warehouse_id + quantity), or
	// multiple via warehouses[] to seed stock across several warehouses in one
	// call. If neither is provided, only the product+variant are created.
	WarehouseID string                   `json:"warehouse_id"`
	Quantity    decimal.Decimal          `json:"quantity"`
	Warehouses  []QuickAddWarehouseStock `json:"warehouses"`
}

// QuickAddWarehouseStock is one warehouse/quantity allocation within QuickAddInput.Warehouses.
type QuickAddWarehouseStock struct {
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
}

// QuickAddStockResult is the stock record created for one warehouse allocation.
type QuickAddStockResult struct {
	WarehouseID string `json:"warehouse_id"`
	StockID     string `json:"stock_id"`
}

type QuickAddResult struct {
	ProductID   string                `json:"product_id"`
	VariantID   string                `json:"variant_id"`
	VariantCode int                   `json:"variant_code"`
	StockID     string                `json:"stock_id"` // first stock record, kept for backward compatibility
	Stocks      []QuickAddStockResult `json:"stocks"`
}

// QuickEditInput updates a variant and/or its parent product's fields in one call.
// All fields are optional pointers; only non-nil fields are updated.
type QuickEditInput struct {
	// Variant fields
	VariantCode *int     `json:"variant_code"`
	VariantName *string  `json:"variant_name"`
	Price       *float64 `json:"price"`
	CostPrice   *float64 `json:"cost_price"`
	HSNCode     *string  `json:"hsn_code"`

	// Product fields
	ProductName *string `json:"product_name"`
	CategoryID  *string `json:"category_id"`
	UOM         *string `json:"uom"`
	Description *string `json:"description"`
}

type QuickEditResult struct {
	ProductID string `json:"product_id"`
	VariantID string `json:"variant_id"`
}

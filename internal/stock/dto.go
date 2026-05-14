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

	// Stock
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
}

type QuickAddResult struct {
	ProductID   string `json:"product_id"`
	VariantID   string `json:"variant_id"`
	VariantCode int    `json:"variant_code"`
	StockID     string `json:"stock_id"`
}

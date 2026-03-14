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

package stock

import "github.com/shopspring/decimal"

type StockCreateInput struct {
	VariantID   string          `json:"variant_id"`
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
}

type StockUpdateInput struct {
	VariantID   string          `json:"variant_id"`
	WarehouseID string          `json:"warehouse_id"`
	Quantity    decimal.Decimal `json:"quantity"`
}

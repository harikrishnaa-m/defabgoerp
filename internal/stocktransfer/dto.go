package stocktransfer

import "github.com/shopspring/decimal"

type CreateStockTransferInput struct {
	FromWarehouseID string              `json:"from_warehouse_id"`
	ToWarehouseID   string              `json:"to_warehouse_id"`
	Reference       string              `json:"reference"`
	Items           []StockTransferItem `json:"items"`
}

type InterWarehouseTransferInput struct {
	FromWarehouseID string                       `json:"from_warehouse_id"`
	ToWarehouseID   string                       `json:"to_warehouse_id"`
	Reason          string                       `json:"reason"`
	Items           []InterWarehouseTransferItem `json:"items"`
}

type InterWarehouseTransferItem struct {
	VariantID string          `json:"variant_id"`
	Quantity  decimal.Decimal `json:"quantity"`
}

type StockTransferItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

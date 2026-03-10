package stocktransfer

type CreateStockTransferInput struct {
	FromWarehouseID string              `json:"from_warehouse_id"`
	ToWarehouseID   string              `json:"to_warehouse_id"`
	Reference       string              `json:"reference"`
	Items           []StockTransferItem `json:"items"`
}

type StockTransferItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

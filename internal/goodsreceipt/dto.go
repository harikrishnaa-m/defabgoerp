package goodsreceipt

type CreateGoodsReceiptInput struct {
	SupplierID  string                   `json:"supplier_id"`
	WarehouseID string                   `json:"warehouse_id"` // CENTRAL warehouse
	Reference   string                   `json:"reference"`    // Invoice / DC no
	Items       []CreateGoodsReceiptItem `json:"items"`
}

type CreateGoodsReceiptItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

package purchase

type CreatePurchaseOrderInput struct {
	SupplierID   string                   `json:"supplier_id"`
	WarehouseID  string                   `json:"warehouse_id"`
	ExpectedDate string                   `json:"expected_date"`
	Items        []CreatePurchaseOrderItem `json:"items"`
}

type CreatePurchaseOrderItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type UpdatePOStatusInput struct {
	Status string `json:"status"`
}

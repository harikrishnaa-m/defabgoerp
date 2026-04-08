package production

type CreateMaterialInput struct {
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

type CreateProductionOrderInput struct {
	OutputVariantID string                `json:"output_variant_id"`
	OutputQuantity  float64               `json:"output_quantity"`
	Notes           string                `json:"notes"`
	Materials       []CreateMaterialInput `json:"materials"`
}

type StatusUpdateInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

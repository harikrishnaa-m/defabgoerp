package onlinestock

type SetStockInput struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateStockInput struct {
	Quantity int `json:"quantity"`
}

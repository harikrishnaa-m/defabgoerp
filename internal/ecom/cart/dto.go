package cart

type AddItemInput struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateQtyInput struct {
	Quantity int `json:"quantity"`
}

package variant

type CreateVariantInput struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	SKU       string  `json:"sku"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`

	AttributeValueIDs []string `json:"attribute_value_ids"`
}

type UpdateVariantInput struct {
	Name      *string  `json:"name"`
	Price     *float64 `json:"price"`
	CostPrice *float64 `json:"cost_price"`
}

type GenerateVariantsInput struct {
	ProductID string     `json:"product_id"`
	BasePrice float64    `json:"base_price"`
	Groups    [][]string `json:"groups"` // attribute value id groups
}

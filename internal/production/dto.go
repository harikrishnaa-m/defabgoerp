package production

type CreateMaterialInput struct {
	RawMaterialStockID string  `json:"raw_material_stock_id"`
	QuantityUsed       float64 `json:"quantity_used"`
}

// NewProductInput is used when creating a brand new product during production.
type NewProductInput struct {
	Name              string `json:"name"`
	CategoryID        string `json:"category_id"`
	Brand             string `json:"brand"`
	Description       string `json:"description"`
	FabricComposition string `json:"fabric_composition"`
	Pattern           string `json:"pattern"`
	Occasion          string `json:"occasion"`
	CareInstructions  string `json:"care_instructions"`
	UOM               string `json:"uom"`
}

// NewVariantInput is used when creating a new variant during production.
type NewVariantInput struct {
	Name              string   `json:"name"`
	Price             float64  `json:"price"`
	CostPrice         float64  `json:"cost_price"`
	AttributeValueIDs []string `json:"attribute_value_ids"`
}

type CreateProductionOrderInput struct {
	// Scenario 1: existing variant
	OutputVariantID string `json:"output_variant_id"`

	// Scenario 2: existing product + new variant
	OutputProductID string           `json:"output_product_id"`
	NewVariant      *NewVariantInput `json:"new_variant"`

	// Scenario 3: new product + new variant
	NewProduct *NewProductInput `json:"new_product"`

	OutputQuantity float64               `json:"output_quantity"`
	Notes          string                `json:"notes"`
	Materials      []CreateMaterialInput `json:"materials"`
}

type StatusUpdateInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

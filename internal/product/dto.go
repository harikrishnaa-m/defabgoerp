package product

type CreateProductInput struct {
	Name       string `json:"name"`
	CategoryID string `json:"category_id"`
	Brand      string `json:"brand"`

	Description       string `json:"description"`
	FabricComposition string `json:"fabric_composition"`
	Pattern           string `json:"pattern"`
	Occasion          string `json:"occasion"`
	CareInstructions  string `json:"care_instructions"`

	IsWebVisible *bool  `json:"is_web_visible"`
	IsStitched   *bool  `json:"is_stitched"`
	UOM          string `json:"uom"`
}

type UpdateProductInput struct {
	Name       *string `json:"name"`
	CategoryID *string `json:"category_id"`
	Brand      *string `json:"brand"`

	Description       *string `json:"description"`
	FabricComposition *string `json:"fabric_composition"`
	Pattern           *string `json:"pattern"`
	Occasion          *string `json:"occasion"`
	CareInstructions  *string `json:"care_instructions"`

	IsWebVisible *bool   `json:"is_web_visible"`
	IsStitched   *bool   `json:"is_stitched"`
	UOM          *string `json:"uom"`
}

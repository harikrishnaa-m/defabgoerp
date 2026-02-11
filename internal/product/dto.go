package product

type CreateProductInput struct {
	Name         string `json:"name"`
	CategoryID   string `json:"category_id"`
	Brand        string `json:"brand"`
	ImageURL     string `json:"image_url"`
	IsWebVisible *bool  `json:"is_web_visible"`
	IsStitched   *bool  `json:"is_stitched"`
	UOM          string `json:"uom"`
}

type UpdateProductInput struct {
	Name         *string `json:"name"`
	CategoryID   *string `json:"category_id"`
	Brand        *string `json:"brand"`
	ImageURL     *string `json:"image_url"`
	IsWebVisible *bool   `json:"is_web_visible"`
	IsStitched   *bool   `json:"is_stitched"`
	UOM          *string `json:"uom"`
}

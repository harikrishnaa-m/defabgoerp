package category

type CreateCategoryInput struct {
	Name     string  `json:"name"`
	ImageURL *string `json:"image_url"`
}

type UpdateCategoryInput struct {
	Name     *string `json:"name"`
	ImageURL *string `json:"image_url"`
}

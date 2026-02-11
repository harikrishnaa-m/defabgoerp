package category

type CreateCategoryInput struct {
	Name string `json:"name"`
}

type UpdateCategoryInput struct {
	Name *string `json:"name"`
}

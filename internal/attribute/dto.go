package attribute

type CreateAttributeInput struct {
	Name string `json:"name"`
}

type UpdateAttributeInput struct {
	Name *string `json:"name"`
}

type CreateValueInput struct {
	AttributeID string `json:"attribute_id"`
	Value       string `json:"value"`
}

type UpdateValueInput struct {
	Value *string `json:"value"`
}

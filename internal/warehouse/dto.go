package warehouse

type CreateWarehouseInput struct {
	BranchID *int   `json:"branch_id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // STORE, CENTRAL, FACTORY
}

type UpdateWarehouseInput struct {
	BranchID *int    `json:"branch_id"`
	Name     *string `json:"name"`
	Type     *string `json:"type"`
}

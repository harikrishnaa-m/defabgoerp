package warehouse

type CreateWarehouseInput struct {
	BranchID      string `json:"branch_id"`
	Name          string `json:"name"`
	Type          string `json:"type"` // STORE, CENTRAL, FACTORY
	WarehouseCode string `json:"warehouse_code"`
}

type UpdateWarehouseInput struct {
	BranchID *string `json:"branch_id"`
	Name     *string `json:"name"`
	Type     *string `json:"type"`
}

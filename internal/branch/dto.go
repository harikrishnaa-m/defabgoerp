package branch

type CreateBranchInput struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	ManagerID string `json:"manager_id"`
}

type UpdateBranchInput struct {
	Name      *string `json:"name"`
	Address   *string `json:"address"`
	ManagerID *string `json:"manager_id"`
}

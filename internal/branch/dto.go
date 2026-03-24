package branch

type CreateBranchInput struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	ManagerID   string `json:"manager_id"`
	City        string `json:"city"`
	State       string `json:"state"`
	PhoneNumber string `json:"phone_number"`
}

type UpdateBranchInput struct {
	Name        *string `json:"name"`
	Address     *string `json:"address"`
	ManagerID   *string `json:"manager_id"`
	PhoneNumber *string `json:"phone_number"`
	City        *string `json:"city"`
	State       *string `json:"state"`
}

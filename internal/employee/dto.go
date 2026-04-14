package employee

type CreateEmployeeInput struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	BranchID *string `json:"branch_id"`
}

type UpdateEmployeeInput struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	BranchID *string `json:"branch_id"`
	IsActive *bool   `json:"is_active"`
}

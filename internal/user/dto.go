package user

type CreateUserInput struct {
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	RoleID   uint    `json:"role_id"`
	BranchID *string `json:"branch_id"`
}

type UpdateUserInput struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	RoleID   *uint   `json:"role_id"`
	BranchID *string `json:"branch_id"`
	IsActive *bool   `json:"is_active"`
}

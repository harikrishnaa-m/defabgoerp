package user

type CreateUserInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   uint   `json:"role_id"`
	BranchID *uint  `json:"branch_id"`
}

type UpdateUserInput struct {
	Name     *string `json:"name"`
	RoleID   *uint   `json:"role_id"`
	BranchID *uint   `json:"branch_id"`
	IsActive *bool   `json:"is_active"`
}

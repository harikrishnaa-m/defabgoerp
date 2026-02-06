package auth

type RegisterInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   uint   `json:"role_id"`
	BranchID *uint  `json:"branch_id"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

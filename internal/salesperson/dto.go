package salesperson

type CreateSalesPersonInput struct {
	BranchID *string `json:"branch_id"`
	Name     string  `json:"name"`
	Phone    string  `json:"phone"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
}

type UpdateSalesPersonInput struct {
	BranchID *string `json:"branch_id"`
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	Email    *string `json:"email"`
}

type SalesFilter struct {
	From       string // YYYY-MM-DD
	To         string // YYYY-MM-DD
	CategoryID string
}

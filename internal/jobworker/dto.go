package jobworker

type CreateJobOrderWorkerInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Role  string `json:"role"` // optional: DESIGNER | CUTTER | STITCHER | HAND_WORKER | ""
}

type UpdateJobOrderWorkerInput struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

package attendance

// PunchInput is used for self-service punch in/out.
type PunchInput struct {
	Notes    string  `json:"notes"`
	BranchID *string `json:"branch_id"`
}

// UploadResult is returned after processing an attendance Excel upload.
type UploadResult struct {
	Date             string            `json:"date"`
	TotalRows        int               `json:"total_rows"`
	RecordsCreated   int               `json:"records_created"`
	EmployeesCreated int               `json:"employees_created"`
	Skipped          int               `json:"skipped"`
	Errors           []string          `json:"errors,omitempty"`
	Details          []UploadRowDetail `json:"details"`
}

// UploadRowDetail describes the outcome for each row in an uploaded Excel.
type UploadRowDetail struct {
	Row             int    `json:"row"`
	EmployeeName    string `json:"employee_name"`
	EmployeeCode    string `json:"employee_code"`
	Status          string `json:"status"` // "created", "skipped", "error", "employee_created"
	Message         string `json:"message,omitempty"`
	PunchesRecorded int    `json:"punches_recorded"`
}

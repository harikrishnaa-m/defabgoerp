package jobworker

import "time"

// JobOrderWorker is a named worker (cutter/stitcher/designer/hand worker/etc.)
// that job order items and work logs reference by ID instead of a free-text name.
// Role is an optional tag only — a worker with Role == "" can be assigned to any field.
type JobOrderWorker struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	RoleDesigner   = "DESIGNER"
	RoleCutter     = "CUTTER"
	RoleStitcher   = "STITCHER"
	RoleHandWorker = "HAND_WORKER"
)

// IsValidRole reports whether role is a known tag, or empty (no fixed role).
func IsValidRole(role string) bool {
	switch role {
	case "", RoleDesigner, RoleCutter, RoleStitcher, RoleHandWorker:
		return true
	}
	return false
}

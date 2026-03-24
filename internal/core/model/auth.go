package model

import (
	"time"

	"github.com/google/uuid"
)

type Role struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"unique;not null" json:"name"`  // "Admin", "Manager", "Cashier"
	Permissions string `gorm:"type:text" json:"permissions"` // JSON blob of scopes
}

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	Email        string    `gorm:"unique;not null;index" json:"email"` // Index for fast login
	PasswordHash string    `gorm:"not null" json:"-"`                  // Never return password in JSON
	RoleID       uint      `json:"role_id"`
	Role         Role      `json:"role"`
	BranchID     *string   `json:"branch_id"` // Nullable for Admin
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

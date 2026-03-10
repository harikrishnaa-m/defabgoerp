package auth

import (
	"database/sql"
	"defab-erp/internal/core/model" // <--- Importing shared models
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

// UpdateRefreshToken updates the user's refresh token in the database
func (s *Store) UpdateRefreshToken(userID interface{}, refreshToken string) error {
	query := `UPDATE users SET refresh_token = $1 WHERE id = $2`
	_, err := s.db.Exec(query, refreshToken, userID)
	return err
}

// UpdateResetToken updates the user's reset token in the database
func (s *Store) UpdateResetToken(userID interface{}, resetToken string) error {
	query := `UPDATE users SET reset_token = $1 WHERE id = $2`
	_, err := s.db.Exec(query, resetToken, userID)
	return err
}

// GetUserByRefreshToken fetches a user by their refresh token
func (s *Store) GetUserByRefreshToken(refreshToken string) (*model.User, error) {
	u := &model.User{}
	var roleID uint
	var roleName string
	var rolePermissions string
	var branchID sql.NullInt64
	query := `
		SELECT u.id, u.name, u.email, u.password_hash, u.role_id, r.id, r.name, r.permissions, u.branch_id, u.is_active, u.created_at
		FROM users u
		JOIN roles r ON u.role_id = r.id
		WHERE u.refresh_token = $1
	`
	err := s.db.QueryRow(query, refreshToken).Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.RoleID,
		&roleID,
		&roleName,
		&rolePermissions,
		&branchID,
		&u.IsActive,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	u.Role.ID = roleID
	u.Role.Name = roleName
	u.Role.Permissions = rolePermissions
	if branchID.Valid {
		id := uint(branchID.Int64)
		u.BranchID = &id
	}
	return u, nil
}

// GetUserByResetToken fetches a user by their reset token
func (s *Store) GetUserByResetToken(resetToken string) (*model.User, error) {
	u := &model.User{}
	var roleID uint
	var roleName string
	var rolePermissions string
	var branchID sql.NullInt64
	query := `
		SELECT u.id, u.name, u.email, u.password_hash, u.role_id, r.id, r.name, r.permissions, u.branch_id, u.is_active, u.created_at
		FROM users u
		JOIN roles r ON u.role_id = r.id
		WHERE u.reset_token = $1
	`
	err := s.db.QueryRow(query, resetToken).Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.RoleID,
		&roleID,
		&roleName,
		&rolePermissions,
		&branchID,
		&u.IsActive,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	u.Role.ID = roleID
	u.Role.Name = roleName
	u.Role.Permissions = rolePermissions
	if branchID.Valid {
		id := uint(branchID.Int64)
		u.BranchID = &id
	}
	return u, nil
}

// UpdatePassword updates the user's password hash
func (s *Store) UpdatePassword(userID interface{}, newHash string) error {
	query := `UPDATE users SET password_hash = $1 WHERE id = $2`
	_, err := s.db.Exec(query, newHash, userID)
	return err
}

// CreateUser creates a new user
func (s *Store) CreateUser(u *model.User) error {
	query := `
		INSERT INTO users
		(name, email, password_hash, role_id, branch_id, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id, created_at, is_active
	`

	err := s.db.QueryRow(
		query,
		u.Name,
		u.Email,
		u.PasswordHash,
		u.RoleID,
		u.BranchID,
	).Scan(&u.ID, &u.CreatedAt, &u.IsActive)

	return err
}

// GetUserByEmail fetches a user by email
func (s *Store) GetUserByEmail(email string) (*model.User, error) {
	fmt.Println("[DEBUG] Looking for user with email:", email)
	u := &model.User{}

	var roleID uint
	var roleName string
	var rolePermissions string
	var branchID sql.NullInt64

	query := `
	SELECT
		u.id,
		u.name,
		u.email,
		u.password_hash,
		u.role_id,
		r.id,
		r.name,
		r.permissions,
		u.branch_id,
		u.is_active,
		u.created_at
	FROM users u
	JOIN roles r ON u.role_id = r.id
	WHERE u.email = $1
	`

	err := s.db.QueryRow(query, email).Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.RoleID,
		&roleID,
		&roleName,
		&rolePermissions,
		&branchID,
		&u.IsActive,
		&u.CreatedAt,
	)

	fmt.Println("[DEBUG] Query error:", err)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	u.Role.ID = roleID
	u.Role.Name = roleName
	u.Role.Permissions = rolePermissions

	if branchID.Valid {
		id := uint(branchID.Int64)
		u.BranchID = &id
	}

	fmt.Println("[DEBUG] User fetched:", u)
	return u, nil
}

// GetUserByID fetches a user by ID
func (s *Store) GetUserByID(id string) (*model.User, error) {
	u := &model.User{}

	var roleID uint
	var roleName string
	var branchID sql.NullInt64

	query := `
	SELECT
		u.id,
		u.name,
		u.email,
		u.password_hash,
		u.role_id,
		r.id,
		r.name,
		u.branch_id,
		u.is_active,
		u.created_at
	FROM users u
	JOIN roles r ON u.role_id = r.id
	WHERE u.id = $1
	`

	err := s.db.QueryRow(query, id).Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.PasswordHash,
		&u.RoleID,
		&roleID,
		&roleName,
		&branchID,
		&u.IsActive,
		&u.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	u.Role.ID = roleID
	u.Role.Name = roleName

	if branchID.Valid {
		b := uint(branchID.Int64)
		u.BranchID = &b
	}

	return u, nil
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

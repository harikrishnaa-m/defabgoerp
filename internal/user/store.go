package user

import (
	"database/sql"
	"defab-erp/internal/core/model"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//
// ✅ CREATE USER (admin create staff)
//

func (s *Store) Create(u *model.User) error {
	query := `
	INSERT INTO users
	(name,email,password_hash,role_id,branch_id,is_active)
	VALUES ($1,$2,$3,$4,$5,TRUE)
	RETURNING id, created_at, is_active
	`

	return s.db.QueryRow(
		query,
		u.Name,
		u.Email,
		u.PasswordHash,
		u.RoleID,
		u.BranchID,
	).Scan(&u.ID, &u.CreatedAt, &u.IsActive)
}



func (s *Store) List() (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
	  u.id,
	  u.name,
	  u.email,
	  u.role_id,
	  r.id,
	  r.name,
	  r.permissions,
	  u.branch_id,
	  u.is_active,
	  u.created_at
	FROM users u
	JOIN roles r ON u.role_id = r.id
	ORDER BY u.created_at DESC
	`)
}



func (s *Store) GetByID(id string) (*model.User, error) {
	u := &model.User{}

	err := s.db.QueryRow(`
	SELECT
	  u.id,
	  u.name,
	  u.email,
	  u.role_id,
	  r.id,
	  r.name,
	  r.permissions,
	  u.branch_id,
	  u.is_active,
	  u.created_at
	FROM users u
	JOIN roles r ON u.role_id = r.id
	WHERE u.id=$1
	`, id).Scan(
		&u.ID,
		&u.Name,
		&u.Email,
		&u.RoleID,
		&u.Role.ID,
		&u.Role.Name,
		&u.Role.Permissions,
		&u.BranchID,
		&u.IsActive,
		&u.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return u, nil
}



func (s *Store) Update(id string, in UpdateUserInput) error {
	_, err := s.db.Exec(`
	UPDATE users SET
	  name = COALESCE($1,name),
	  role_id = COALESCE($2,role_id),
	  branch_id = COALESCE($3,branch_id),
	  is_active = COALESCE($4,is_active)
	WHERE id=$5
	`,
		in.Name,
		in.RoleID,
		in.BranchID,
		in.IsActive,
		id,
	)

	return err
}


func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE users SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}


func (s *Store) ListActive(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
	  u.id,
	  u.name,
	  u.email,
	  u.role_id,
	  r.id,
	  r.name,
	  r.permissions,
	  u.branch_id,
	  u.is_active,
	  u.created_at
	FROM users u
	JOIN roles r ON u.role_id = r.id
	WHERE u.is_active = TRUE
	ORDER BY u.created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
}


func (s *Store) CountActive() (int, error) {
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM users WHERE is_active = TRUE`,
	).Scan(&total)

	return total, err
}

package branch

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(name, address, managerID, branchCode, city, state, phoneNumber string) error {
	_, err := s.db.Exec(
		`INSERT INTO branches (name, address, manager_id, branch_code, city, state, phone_number)
		 VALUES ($1,$2, NULLIF($3,'')::uuid, $4, $5, $6, $7)`,
		name, address, managerID, branchCode, city, state, phoneNumber,
	)
	return err
}

// Generate next branch code (BRxxx)
func (s *Store) NextBranchCode() (string, error) {
	var maxCode sql.NullString
	err := s.db.QueryRow(`SELECT MAX(branch_code) FROM branches WHERE branch_code LIKE 'BR%'`).Scan(&maxCode)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return "", err
	}
	nextNum := 1
	if maxCode.Valid {
		var num int
		_, err := fmt.Sscanf(maxCode.String, "BR%03d", &num)
		if err == nil {
			nextNum = num + 1
		}
	}
	return fmt.Sprintf("BR%03d", nextNum), nil
}

func (s *Store) List() (*sql.Rows, error) {
	return s.db.Query(
		`SELECT b.id, b.name, b.address, b.manager_id, u.name as manager_name, b.created_at,
				b.branch_code, b.phone_number, b.city, b.state
		 FROM branches b
		 LEFT JOIN users u ON b.manager_id = u.id
		 ORDER BY b.id`,
	)
}

func (s *Store) Update(id string, in UpdateBranchInput) error {
	_, err := s.db.Exec(`
		UPDATE branches
		SET
		  name = COALESCE($1, name),
		  address = COALESCE($2, address),
		  manager_id = COALESCE(NULLIF($3,'')::uuid, manager_id),
		  phone_number = COALESCE($4, phone_number),
		  city = COALESCE($5, city),
		  state = COALESCE($6, state)
		WHERE id = $7::uuid
	`,
		in.Name,
		in.Address,
		in.ManagerID,
		in.PhoneNumber,
		in.City,
		in.State,
		id,
	)
	return err
}

func (s *Store) GetByID(id string) *sql.Row {
	return s.db.QueryRow(
		`SELECT b.id, b.name, b.address, b.manager_id, u.name as manager_name, b.created_at,
				b.branch_code, b.phone_number, b.city, b.state
		 FROM branches b
		 LEFT JOIN users u ON b.manager_id = u.id
		 WHERE b.id = $1`, id)

}

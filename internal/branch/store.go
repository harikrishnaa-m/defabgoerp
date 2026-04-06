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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var branchID string
	err = tx.QueryRow(
		`INSERT INTO branches (name, address, manager_id, branch_code, city, state, phone_number)
		 VALUES ($1,$2, NULLIF($3,'')::uuid, $4, $5, $6, $7) RETURNING id`,
		name, address, managerID, branchCode, city, state, phoneNumber,
	).Scan(&branchID)
	if err != nil {
		return err
	}

	// Sync branch_id onto the assigned manager
	if managerID != "" {
		_, err = tx.Exec(
			`UPDATE users SET branch_id = $1::uuid WHERE id = $2::uuid`,
			branchID, managerID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// If manager is being changed, clear branch_id from the old manager first
	if in.ManagerID != nil && *in.ManagerID != "" {
		_, err = tx.Exec(
			`UPDATE users SET branch_id = NULL WHERE branch_id = $1::uuid`,
			id,
		)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
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
	if err != nil {
		return err
	}

	// Set branch_id on the new manager
	if in.ManagerID != nil && *in.ManagerID != "" {
		_, err = tx.Exec(
			`UPDATE users SET branch_id = $1::uuid WHERE id = $2::uuid`,
			id, *in.ManagerID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetByID(id string) *sql.Row {
	return s.db.QueryRow(
		`SELECT b.id, b.name, b.address, b.manager_id, u.name as manager_name, b.created_at,
				b.branch_code, b.phone_number, b.city, b.state
		 FROM branches b
		 LEFT JOIN users u ON b.manager_id = u.id
		 WHERE b.id = $1`, id)

}

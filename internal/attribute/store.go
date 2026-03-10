package attribute

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//
// ATTRIBUTE
//

func (s *Store) Create(name string) error {
	_, err := s.db.Exec(
		`INSERT INTO attributes (name) VALUES ($1)`,
		name,
	)
	return err
}

func (s *Store) List(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id, name, is_active
	FROM attributes
	WHERE is_active = TRUE
	ORDER BY name
	LIMIT $1 OFFSET $2
	`, limit, offset)
}

func (s *Store) Update(id string, name *string) error {
	_, err := s.db.Exec(`
	UPDATE attributes
	SET name = COALESCE($1,name)
	WHERE id=$2
	`, name, id)
	return err
}

func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE attributes SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}

//
// ATTRIBUTE VALUES
//

func (s *Store) CreateValue(attID, value string) error {
	_, err := s.db.Exec(`
	INSERT INTO attribute_values (attribute_id,value)
	VALUES ($1,$2)
	`, attID, value)
	return err
}

func (s *Store) ListValues(attID string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id, value, is_active
	FROM attribute_values
	WHERE attribute_id=$1 AND is_active=TRUE
	ORDER BY value
	`, attID)
}

func (s *Store) SetValueActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE attribute_values SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}

func (s *Store) UpdateValue(id string, value *string) error {
	_, err := s.db.Exec(`
	UPDATE attribute_values
	SET value = COALESCE($1,value)
	WHERE id=$2
	`, value, id)

	return err
}

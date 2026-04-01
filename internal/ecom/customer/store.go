package customer

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ── Registration / Auth ─────────────────────────────────────

func (s *Store) EmailExists(email string) bool {
	var exists bool
	s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM ecom_customers WHERE email = $1)`, email).Scan(&exists)
	return exists
}

func (s *Store) Create(name, email, phone, passwordHash string) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO ecom_customers (name, email, phone, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, name, email, phone, passwordHash).Scan(&id)
	return id, err
}

func (s *Store) GetByEmail(email string) (id, name, passwordHash string, err error) {
	err = s.db.QueryRow(`
		SELECT id, name, COALESCE(password_hash, '') FROM ecom_customers
		WHERE email = $1 AND is_active = true
	`, email).Scan(&id, &name, &passwordHash)
	return
}

// GetByGoogleID finds a customer by their Google account ID.
func (s *Store) GetByGoogleID(googleID string) (id, name, email string, err error) {
	err = s.db.QueryRow(`
		SELECT id, name, email FROM ecom_customers
		WHERE google_id = $1 AND is_active = true
	`, googleID).Scan(&id, &name, &email)
	return
}

// CreateGoogleUser creates a new customer from Google sign-in (no password).
func (s *Store) CreateGoogleUser(name, email, googleID string) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO ecom_customers (name, email, google_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, name, email, googleID).Scan(&id)
	return id, err
}

// LinkGoogleID links a Google account to an existing customer.
func (s *Store) LinkGoogleID(customerID, googleID string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_customers SET google_id = $2, updated_at = NOW()
		WHERE id = $1
	`, customerID, googleID)
	return err
}

// SetResetToken stores a password reset token with 1-hour expiry.
func (s *Store) SetResetToken(email, token string) error {
	result, err := s.db.Exec(`
		UPDATE ecom_customers SET reset_token = $2, reset_token_expiry = NOW() + INTERVAL '1 hour', updated_at = NOW()
		WHERE email = $1 AND is_active = true
	`, email, token)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetByResetToken finds a customer by a valid (non-expired) reset token.
func (s *Store) GetByResetToken(token string) (id string, err error) {
	err = s.db.QueryRow(`
		SELECT id FROM ecom_customers
		WHERE reset_token = $1 AND reset_token_expiry > NOW() AND is_active = true
	`, token).Scan(&id)
	return
}

// UpdatePassword sets a new password hash and clears the reset token.
func (s *Store) UpdatePassword(customerID, passwordHash string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_customers SET password_hash = $2, reset_token = NULL, reset_token_expiry = NULL, updated_at = NOW()
		WHERE id = $1
	`, customerID, passwordHash)
	return err
}

// GetPasswordHash returns the current password hash for a customer.
func (s *Store) GetPasswordHash(customerID string) (string, error) {
	var hash sql.NullString
	err := s.db.QueryRow(`SELECT password_hash FROM ecom_customers WHERE id = $1`, customerID).Scan(&hash)
	if err != nil {
		return "", err
	}
	if !hash.Valid || hash.String == "" {
		return "", nil
	}
	return hash.String, nil
}

func (s *Store) GetProfile(customerID string) (map[string]interface{}, error) {
	var name, email string
	var phone sql.NullString
	var createdAt time.Time

	err := s.db.QueryRow(`
		SELECT name, email, phone, created_at FROM ecom_customers WHERE id = $1
	`, customerID).Scan(&name, &email, &phone, &createdAt)
	if err != nil {
		return nil, err
	}

	p := ""
	if phone.Valid {
		p = phone.String
	}

	return map[string]interface{}{
		"id":         customerID,
		"name":       name,
		"email":      email,
		"phone":      p,
		"created_at": createdAt,
	}, nil
}

func (s *Store) UpdateProfile(customerID string, in UpdateProfileInput) error {
	_, err := s.db.Exec(`
		UPDATE ecom_customers SET name = $2, phone = $3, updated_at = NOW()
		WHERE id = $1
	`, customerID, in.Name, in.Phone)
	return err
}

// ── Addresses ───────────────────────────────────────────────

func (s *Store) AddAddress(customerID string, in AddressInput) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// If this is set as default, clear existing defaults
	if in.IsDefault {
		tx.Exec(`UPDATE ecom_addresses SET is_default = false WHERE customer_id = $1`, customerID)
	}

	// If it's the first address, make it default automatically
	var count int
	tx.QueryRow(`SELECT COUNT(*) FROM ecom_addresses WHERE customer_id = $1`, customerID).Scan(&count)
	if count == 0 {
		in.IsDefault = true
	}

	var id string
	err = tx.QueryRow(`
		INSERT INTO ecom_addresses (customer_id, label, full_name, phone, address_line1, address_line2, city, state, pincode, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, customerID, in.Label, in.FullName, in.Phone, in.AddressLine1, in.AddressLine2,
		in.City, in.State, in.Pincode, in.IsDefault).Scan(&id)
	if err != nil {
		return "", err
	}

	return id, tx.Commit()
}

func (s *Store) ListAddresses(customerID string) ([]Address, error) {
	rows, err := s.db.Query(`
		SELECT id, label, full_name, phone, address_line1, COALESCE(address_line2, ''),
		       city, state, pincode, is_default, created_at
		FROM ecom_addresses
		WHERE customer_id = $1
		ORDER BY is_default DESC, created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addrs []Address
	for rows.Next() {
		var a Address
		if err := rows.Scan(&a.ID, &a.Label, &a.FullName, &a.Phone,
			&a.AddressLine1, &a.AddressLine2, &a.City, &a.State,
			&a.Pincode, &a.IsDefault, &a.CreatedAt); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func (s *Store) UpdateAddress(customerID, addressID string, in AddressInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if in.IsDefault {
		tx.Exec(`UPDATE ecom_addresses SET is_default = false WHERE customer_id = $1`, customerID)
	}

	res, err := tx.Exec(`
		UPDATE ecom_addresses
		SET label = $3, full_name = $4, phone = $5, address_line1 = $6, address_line2 = $7,
		    city = $8, state = $9, pincode = $10, is_default = $11
		WHERE id = $1 AND customer_id = $2
	`, addressID, customerID, in.Label, in.FullName, in.Phone,
		in.AddressLine1, in.AddressLine2, in.City, in.State, in.Pincode, in.IsDefault)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("address not found")
	}
	return tx.Commit()
}

func (s *Store) DeleteAddress(customerID, addressID string) error {
	res, err := s.db.Exec(`
		DELETE FROM ecom_addresses WHERE id = $1 AND customer_id = $2
	`, addressID, customerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("address not found")
	}
	return nil
}

func (s *Store) GetAddress(customerID, addressID string) (*Address, error) {
	var a Address
	err := s.db.QueryRow(`
		SELECT id, label, full_name, phone, address_line1, COALESCE(address_line2, ''),
		       city, state, pincode, is_default, created_at
		FROM ecom_addresses
		WHERE id = $1 AND customer_id = $2
	`, addressID, customerID).Scan(&a.ID, &a.Label, &a.FullName, &a.Phone,
		&a.AddressLine1, &a.AddressLine2, &a.City, &a.State,
		&a.Pincode, &a.IsDefault, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

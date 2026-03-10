package coupon

import (
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CREATE
func (s *Store) Create(in CreateCouponInput) (string, error) {
	var id string

	err := s.db.QueryRow(`
		INSERT INTO coupons
		(code, description, discount_type, discount_value,
		 min_order_value, max_discount_amount,
		 start_date, end_date, usage_limit, usage_per_customer, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,true)
		RETURNING id
	`,
		in.Code,
		in.Description,
		in.DiscountType,
		in.DiscountValue,
		in.MinOrderValue,
		in.MaxDiscountAmount,
		in.StartDate,
		in.EndDate,
		in.UsageLimit,
		in.UsagePerCustomer,
	).Scan(&id)

	return id, err
}

// LIST
func (s *Store) List(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, code, discount_type, discount_value,
		       start_date, end_date, is_active, created_at
		FROM coupons
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
}

// GET
func (s *Store) Get(id string) *sql.Row {
	return s.db.QueryRow(`
		SELECT id, code, description, discount_type, discount_value,
		       min_order_value, max_discount_amount,
		       start_date, end_date, usage_limit,
		       usage_per_customer, is_active, created_at
		FROM coupons WHERE id=$1
	`, id)
}

// UPDATE
func (s *Store) Update(id string, in UpdateCouponInput) error {
	_, err := s.db.Exec(`
		UPDATE coupons SET
		  description = COALESCE($1, description),
		  discount_type = COALESCE($2, discount_type),
		  discount_value = COALESCE($3, discount_value),
		  min_order_value = COALESCE($4, min_order_value),
		  max_discount_amount = COALESCE($5, max_discount_amount),
		  start_date = COALESCE($6, start_date),
		  end_date = COALESCE($7, end_date),
		  usage_limit = COALESCE($8, usage_limit),
		  usage_per_customer = COALESCE($9, usage_per_customer)
		WHERE id=$10
	`,
		in.Description,
		in.DiscountType,
		in.DiscountValue,
		in.MinOrderValue,
		in.MaxDiscountAmount,
		in.StartDate,
		in.EndDate,
		in.UsageLimit,
		in.UsagePerCustomer,
		id,
	)
	return err
}

// ACTIVATE / DEACTIVATE
func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(`
		UPDATE coupons SET is_active=$1 WHERE id=$2
	`, active, id)
	return err
}








func (s *Store) AttachVariants(couponID string, variantIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, vid := range variantIDs {
		_, err := tx.Exec(`
			INSERT INTO coupon_variants (coupon_id, variant_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, couponID, vid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}






func (s *Store) AttachCategories(couponID string, categoryIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, cid := range categoryIDs {
		_, err := tx.Exec(`
			INSERT INTO coupon_categories (coupon_id, category_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, couponID, cid)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}




func (s *Store) RemoveVariant(mappingID string) error {
	_, err := s.db.Exec(`
		DELETE FROM coupon_variants WHERE id = $1
	`, mappingID)
	return err
}



func (s *Store) RemoveCategory(mappingID string) error {
	_, err := s.db.Exec(`
		DELETE FROM coupon_categories WHERE id = $1
	`, mappingID)
	return err
}

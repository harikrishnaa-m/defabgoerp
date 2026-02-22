package stockrequest

import (
	"database/sql"
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}


//   Create Stock Request


func (s *Store) CreateRequest(
	fromWarehouse, toWarehouse, requestedBy string,
	expectedDate *string,
) (string, error) {

	var id string

	err := s.db.QueryRow(`
	INSERT INTO stock_requests
	(from_warehouse_id, to_warehouse_id, requested_by, expected_date)
	VALUES ($1,$2,$3,$4)
	RETURNING id
	`,
		fromWarehouse,
		toWarehouse,
		requestedBy,
		expectedDate,
	).Scan(&id)

	return id, err
}


//  Add Request Items

func (s *Store) AddItem(
	requestID, variantID string,
	qty int,
) error {

	_, err := s.db.Exec(`
	INSERT INTO stock_request_items
	(stock_request_id, variant_id, requested_qty)
	VALUES ($1,$2,$3)
	`,
		requestID,
		variantID,
		qty,
	)

	return err
}

//   List Requests 

func (s *Store) ListRequests(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id, status, priority, created_at
	FROM stock_requests
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
}


// Get Request Detail

func (s *Store) GetRequest(id string) (*sql.Row, *sql.Rows) {

	req := s.db.QueryRow(`
	SELECT
		id, status, priority,
		from_warehouse_id, to_warehouse_id,
		expected_date, created_at
	FROM stock_requests
	WHERE id=$1
	`, id)

	items, _ := s.db.Query(`
	SELECT
		variant_id,
		requested_qty,
		approved_qty
	FROM stock_request_items
	WHERE stock_request_id=$1
	`, id)

	return req, items
}

// Approve / Partial / Reject


func isValidStatusTransition(current, next string) bool {

	switch current {
	case "PENDING":
		return next == "APPROVED" || next == "REJECTED"

	case "APPROVED":
		return next == "PARTIAL" || next == "REJECTED"

	case "PARTIAL":
		return next == "PARTIAL" || next == "COMPLETED"

	default:
		return false
	}
}



func (s *Store) UpdateStatus(
	requestID, newStatus, approvedBy, remarks string,
) error {

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentStatus string

	// 🔒 Lock row
	err = tx.QueryRow(`
		SELECT status
		FROM stock_requests
		WHERE id = $1
		FOR UPDATE
	`, requestID).Scan(&currentStatus)

	if err != nil {
		return err
	}

	// 🚫 Block closed requests
	if currentStatus == "COMPLETED" ||
		currentStatus == "CANCELLED" ||
		currentStatus == "REJECTED" {

		return errors.New("stock request already closed")
	}

	// 🚦 Validate transitions
	if !isValidStatusTransition(currentStatus, newStatus) {
		return errors.New("invalid status transition")
	}

	// ✅ Update status
	_, err = tx.Exec(`
		UPDATE stock_requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, newStatus, requestID)
	if err != nil {
		return err
	}

	// 📝 Audit trail
	_, err = tx.Exec(`
		INSERT INTO stock_request_approvals
		(stock_request_id, action, approved_by, remarks)
		VALUES ($1,$2,$3,$4)
	`, requestID, newStatus, approvedBy, remarks)
	if err != nil {
		return err
	}

	return tx.Commit()
}



func (s *Store) ListFiltered(
	status *string,
	fromDate *string,
	toDate *string,
	limit int,
	offset int,
) (*sql.Rows, error) {

	query := `
	SELECT
		sr.id,
		sr.status,
		sr.priority,
		sr.from_warehouse_id,
		sr.to_warehouse_id,
		sr.created_at
	FROM stock_requests sr
	WHERE
		($1::text IS NULL OR sr.status = $1)
		AND ($2::date IS NULL OR sr.created_at::date >= $2)
		AND ($3::date IS NULL OR sr.created_at::date <= $3)
	ORDER BY sr.created_at DESC
	LIMIT $4 OFFSET $5
	`

	return s.db.Query(
		query,
		status,
		fromDate,
		toDate,
		limit,
		offset,
	)
}


func (s *Store) CountFiltered(
	status *string,
	fromDate *string,
	toDate *string,
) (int, error) {

	var total int

	query := `
	SELECT COUNT(*)
	FROM stock_requests
	WHERE
		($1::text IS NULL OR status = $1)
		AND ($2::date IS NULL OR created_at::date >= $2)
		AND ($3::date IS NULL OR created_at::date <= $3)
	`

	err := s.db.QueryRow(
		query,
		status,
		fromDate,
		toDate,
	).Scan(&total)

	return total, err
}

func (s *Store) GetFromWarehouse(requestID string) (string, error) {
	var fromWarehouseID string

	err := s.db.QueryRow(`
		SELECT from_warehouse_id
		FROM stock_requests
		WHERE id = $1
	`, requestID).Scan(&fromWarehouseID)

	if err != nil {
		return "", err
	}

	return fromWarehouseID, nil
}



// func (s *Store) Dispatch(
// 	requestID string,
// 	fromWarehouseID string,
// 	userID string,
// 	 items []DispatchItem,
// 	remarks string,
// ) error {

// 	tx, err := s.db.Begin()
// 	if err != nil {
// 		return err
// 	}
// 	defer tx.Rollback()

// 	for _, it := range items {

// 		// 1️⃣ Check stock
// 		var available int
// 		err := tx.QueryRow(`
// 			SELECT quantity FROM stocks
// 			WHERE warehouse_id = $1 AND variant_id = $2
// 			FOR UPDATE
// 		`, fromWarehouseID, it.VariantID).Scan(&available)

// 		if err != nil {
// 			return err
// 		}

// 		if available < it.Qty {
// 			return errors.New("insufficient stock")
// 		}

// 		// 2️⃣ Deduct stock
// 		_, err = tx.Exec(`
// 			UPDATE stocks
// 			SET quantity = quantity - $1, updated_at = NOW()
// 			WHERE warehouse_id = $2 AND variant_id = $3
// 		`, it.Qty, fromWarehouseID, it.VariantID)
// 		if err != nil {
// 			return err
// 		}

// 		// 3️⃣ Stock movement (OUT)
// 		_, err = tx.Exec(`
// 			INSERT INTO stock_movements
// 			(variant_id, from_warehouse_id, quantity, movement_type, reference)
// 			VALUES ($1,$2,$3,'OUT',$4)
// 		`, it.VariantID, fromWarehouseID, it.Qty, requestID)
// 		if err != nil {
// 			return err
// 		}

// 		// 4️⃣ Update approved qty
// 		_, err = tx.Exec(`
// 			UPDATE stock_request_items
// 			SET approved_qty = approved_qty + $1
// 			WHERE stock_request_id = $2 AND variant_id = $3
// 		`, it.Qty, requestID, it.VariantID)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// 5️⃣ Update request status
// 	_, err = tx.Exec(`
// 		UPDATE stock_requests
// 		SET status = 'IN_TRANSIT', updated_at = NOW()
// 		WHERE id = $1
// 	`, requestID)

// 	if err != nil {
// 		return err
// 	}

// 	return tx.Commit()
// }

func (s *Store) Dispatch(
	requestID string,
	fromWarehouseID string,
	userID string,
	items []DispatchItem,
	remarks string,
) error {

	// 🔒 basic validation
	if requestID == "" {
		return errors.New("invalid stock request id")
	}
	if fromWarehouseID == "" {
		return errors.New("from warehouse id is required")
	}
	if len(items) == 0 {
		return errors.New("no items to dispatch")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1️⃣ Lock request row
	var status string
	err = tx.QueryRow(`
		SELECT status
		FROM stock_requests
		WHERE id = $1
		FOR UPDATE
	`, requestID).Scan(&status)

	if err != nil {
		return err
	}

	// ❌ Block closed requests
	if status == "COMPLETED" || status == "CANCELLED" || status == "REJECTED" {
		return errors.New("stock request already closed")
	}

	// 2️⃣ Dispatch each item
	for _, item := range items {

		if item.VariantID == "" {
			return errors.New("invalid variant id")
		}
		if item.Qty <= 0 {
			return errors.New("dispatch qty must be greater than zero")
		}

		var requestedQty, approvedQty int

		// 🔒 Lock request item row
		err = tx.QueryRow(`
			SELECT requested_qty, approved_qty
			FROM stock_request_items
			WHERE stock_request_id = $1
			AND variant_id = $2
			FOR UPDATE
		`, requestID, item.VariantID).Scan(&requestedQty, &approvedQty)

		if err != nil {
			return err
		}

		remaining := requestedQty - approvedQty
		if remaining <= 0 {
			return fmt.Errorf(
				"no remaining quantity for variant %s",
				item.VariantID,
			)
		}

		if item.Qty > remaining {
			return fmt.Errorf(
				"dispatch qty exceeds remaining for variant %s (remaining %d)",
				item.VariantID,
				remaining,
			)
		}

		// 3️⃣ Reduce stock from source warehouse (atomic)
		res, err := tx.Exec(`
			UPDATE stocks
			SET quantity = quantity - $1
			WHERE variant_id = $2
			AND warehouse_id = $3
			AND quantity >= $1
		`,
			item.Qty,
			item.VariantID,
			fromWarehouseID,
		)
		if err != nil {
			return err
		}

		rows, _ := res.RowsAffected()
		if rows == 0 {
			return fmt.Errorf(
				"insufficient stock for variant %s",
				item.VariantID,
			)
		}

		// 4️⃣ Increase approved qty
		_, err = tx.Exec(`
			UPDATE stock_request_items
			SET approved_qty = approved_qty + $1
			WHERE stock_request_id = $2
			AND variant_id = $3
		`,
			item.Qty,
			requestID,
			item.VariantID,
		)
		if err != nil {
			return err
		}

		// 5️⃣ Insert stock movement (TRANSFER)
		_, err = tx.Exec(`
			INSERT INTO stock_movements (
				variant_id,
				from_warehouse_id,
				to_warehouse_id,
				quantity,
				movement_type,
				stock_request_id,
				status,
				created_at
			)
			SELECT
				$1,
				$2,
				to_warehouse_id,
				$3,
				'TRANSFER',
				$4,
				'IN_TRANSIT',
				NOW()
			FROM stock_requests
			WHERE id = $4
		`,
			item.VariantID,
			fromWarehouseID,
			item.Qty,
			requestID,
		)
		if err != nil {
			return err
		}
	}

	// 6️⃣ Update request status (PARTIAL / COMPLETED)
	var pending int
	err = tx.QueryRow(`
		SELECT COUNT(*)
		FROM stock_request_items
		WHERE stock_request_id = $1
		AND requested_qty > approved_qty
	`, requestID).Scan(&pending)

	if err != nil {
		return err
	}

	newStatus := "PARTIAL"
	if pending == 0 {
		newStatus = "COMPLETED"
	}

	_, err = tx.Exec(`
		UPDATE stock_requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, newStatus, requestID)

	if err != nil {
		return err
	}

	return tx.Commit()
}
package stockrequest

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetCentralWarehouseID() (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM warehouses WHERE type = 'CENTRAL' LIMIT 1`).Scan(&id)
	return id, err
}

func (s *Store) GetWarehouseByBranch(branchID string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM warehouses WHERE branch_id = $1 LIMIT 1`, branchID).Scan(&id)
	return id, err
}

//   Create Stock Request

func (s *Store) CreateRequest(
	fromWarehouse, toWarehouse, requestedBy string,
	priority string,
	expectedDate *string,
) (string, error) {

	if priority == "" {
		priority = "MEDIUM"
	}

	var id string

	err := s.db.QueryRow(`
	INSERT INTO stock_requests
	(from_warehouse_id, to_warehouse_id, requested_by, priority, expected_date)
	VALUES ($1,$2,$3,$4,$5)
	RETURNING id
	`,
		fromWarehouse,
		toWarehouse,
		requestedBy,
		priority,
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

// Get Request Detail with human-readable names

func (s *Store) GetByID(id string) (fiber.Map, error) {
	var (
		srID, status, priority, createdAt      string
		fromWhID, fromWhName, toWhID, toWhName string
		requestedByID, requestedByName         string
		expectedDate                           sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT
			sr.id, sr.status, sr.priority,
			sr.from_warehouse_id, fw.name AS from_warehouse_name,
			sr.to_warehouse_id, tw.name AS to_warehouse_name,
			sr.requested_by, u.name AS requested_by_name,
			sr.expected_date,
			sr.created_at
		FROM stock_requests sr
		JOIN warehouses fw ON fw.id = sr.from_warehouse_id
		JOIN warehouses tw ON tw.id = sr.to_warehouse_id
		JOIN users u ON u.id = sr.requested_by
		WHERE sr.id = $1
	`, id).Scan(
		&srID, &status, &priority,
		&fromWhID, &fromWhName,
		&toWhID, &toWhName,
		&requestedByID, &requestedByName,
		&expectedDate,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Fetch items with variant/product names
	itemRows, err := s.db.Query(`
		SELECT
			sri.id,
			sri.variant_id, v.name AS variant_name, v.sku,
			p.id AS product_id, p.name AS product_name,
			sri.requested_qty, sri.approved_qty,
			COALESCE(sri.remarks, '') AS remarks
		FROM stock_request_items sri
		JOIN variants v ON v.id = sri.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE sri.stock_request_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []fiber.Map
	for itemRows.Next() {
		var itemID, varID, varName, sku, prodID, prodName, remarks string
		var reqQty, appQty float64
		if err := itemRows.Scan(&itemID, &varID, &varName, &sku, &prodID, &prodName, &reqQty, &appQty, &remarks); err != nil {
			return nil, err
		}
		items = append(items, fiber.Map{
			"id":            itemID,
			"variant_id":    varID,
			"variant_name":  varName,
			"sku":           sku,
			"product_id":    prodID,
			"product_name":  prodName,
			"requested_qty": reqQty,
			"approved_qty":  appQty,
			"remarks":       remarks,
		})
	}

	// Fetch approval history
	approvalRows, err := s.db.Query(`
		SELECT
			sra.id, sra.action,
			sra.approved_by, u.name AS approved_by_name,
			COALESCE(sra.remarks, '') AS remarks,
			sra.created_at
		FROM stock_request_approvals sra
		JOIN users u ON u.id = sra.approved_by
		WHERE sra.stock_request_id = $1
		ORDER BY sra.created_at
	`, id)
	if err != nil {
		return nil, err
	}
	defer approvalRows.Close()

	var approvals []fiber.Map
	for approvalRows.Next() {
		var aID, action, approverID, approverName, aRemarks, aCreated string
		if err := approvalRows.Scan(&aID, &action, &approverID, &approverName, &aRemarks, &aCreated); err != nil {
			return nil, err
		}
		approvals = append(approvals, fiber.Map{
			"id":               aID,
			"action":           action,
			"approved_by":      approverID,
			"approved_by_name": approverName,
			"remarks":          aRemarks,
			"created_at":       aCreated,
		})
	}

	result := fiber.Map{
		"id":                  srID,
		"status":              status,
		"priority":            priority,
		"from_warehouse_id":   fromWhID,
		"from_warehouse_name": fromWhName,
		"to_warehouse_id":     toWhID,
		"to_warehouse_name":   toWhName,
		"requested_by":        requestedByID,
		"requested_by_name":   requestedByName,
		"expected_date":       nil,
		"created_at":          createdAt,
		"items":               items,
		"approvals":           approvals,
	}
	if expectedDate.Valid {
		result["expected_date"] = expectedDate.String
	}

	return result, nil
}

// Approve / Partial / Reject

func isValidStatusTransition(current, next string) bool {

	switch current {
	case "PENDING":
		return next == "APPROVED" || next == "REJECTED" || next == "CANCELLED"

	case "APPROVED":
		return next == "REJECTED" || next == "CANCELLED"

	case "PARTIAL_DISPATCH":
		return next == "CANCELLED"

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

	// 🚫 Block rejection after any stock has been dispatched
	if newStatus == "REJECTED" {
		var dispatched float64
		err = tx.QueryRow(`
			SELECT COALESCE(SUM(approved_qty), 0)
			FROM stock_request_items
			WHERE stock_request_id = $1
		`, requestID).Scan(&dispatched)
		if err != nil {
			return err
		}
		if dispatched > 0 {
			return errors.New("cannot reject: stock has already been dispatched")
		}
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
		sr.from_warehouse_id, fw.name AS from_warehouse_name,
		sr.to_warehouse_id, tw.name AS to_warehouse_name,
		sr.requested_by, u.name AS requested_by_name,
		COALESCE(sr.expected_date::text, '') AS expected_date,
		sr.created_at
	FROM stock_requests sr
	JOIN warehouses fw ON fw.id = sr.from_warehouse_id
	JOIN warehouses tw ON tw.id = sr.to_warehouse_id
	JOIN users u ON u.id = sr.requested_by
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

func (s *Store) ListFilteredByBranch(
	branchID string,
	status *string,
	fromDate *string,
	toDate *string,
	limit int,
	offset int,
) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			sr.id,
			sr.status,
			sr.priority,
			sr.from_warehouse_id, fw.name AS from_warehouse_name,
			sr.to_warehouse_id, tw.name AS to_warehouse_name,
			sr.requested_by, u.name AS requested_by_name,
			COALESCE(sr.expected_date::text, '') AS expected_date,
			sr.created_at
		FROM stock_requests sr
		JOIN warehouses fw ON fw.id = sr.from_warehouse_id
		JOIN warehouses tw ON tw.id = sr.to_warehouse_id
		JOIN users u ON u.id = sr.requested_by
		WHERE
			(fw.branch_id = $1 OR tw.branch_id = $1)
			AND ($2::text IS NULL OR sr.status = $2)
			AND ($3::date IS NULL OR sr.created_at::date >= $3)
			AND ($4::date IS NULL OR sr.created_at::date <= $4)
		ORDER BY sr.created_at DESC
		LIMIT $5 OFFSET $6
	`, branchID, status, fromDate, toDate, limit, offset)
}

func (s *Store) CountFilteredByBranch(
	branchID string,
	status *string,
	fromDate *string,
	toDate *string,
) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM stock_requests sr
		JOIN warehouses fw ON fw.id = sr.from_warehouse_id
		JOIN warehouses tw ON tw.id = sr.to_warehouse_id
		WHERE
			(fw.branch_id = $1 OR tw.branch_id = $1)
			AND ($2::text IS NULL OR sr.status = $2)
			AND ($3::date IS NULL OR sr.created_at::date >= $3)
			AND ($4::date IS NULL OR sr.created_at::date <= $4)
	`, branchID, status, fromDate, toDate).Scan(&total)
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

		var requestedQty, approvedQty float64

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

		remaining := int(requestedQty) - int(approvedQty)
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

	newStatus := "PARTIAL_DISPATCH"
	if pending == 0 {
		newStatus = "FULL_DISPATCH"
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

// Receive confirms receipt of dispatched stock at the destination warehouse.
// Upserts stock into branch warehouse and marks movements as RECEIVED.
func (s *Store) Receive(requestID, userID string) error {
	if requestID == "" {
		return errors.New("invalid stock request id")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1️⃣ Get destination warehouse from the request
	var toWarehouseID string
	var status string
	err = tx.QueryRow(`
		SELECT to_warehouse_id, status
		FROM stock_requests
		WHERE id = $1
		FOR UPDATE
	`, requestID).Scan(&toWarehouseID, &status)
	if err != nil {
		return err
	}

	if status != "PARTIAL_DISPATCH" && status != "FULL_DISPATCH" {
		return errors.New("no dispatched stock to receive")
	}

	// 2️⃣ Get all IN_TRANSIT movements for this request
	rows, err := tx.Query(`
		SELECT id, variant_id, quantity
		FROM stock_movements
		WHERE stock_request_id = $1
		AND status = 'IN_TRANSIT'
		FOR UPDATE
	`, requestID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type movementRow struct {
		ID        string
		VariantID string
		Quantity  float64
	}

	var movements []movementRow
	for rows.Next() {
		var m movementRow
		if err := rows.Scan(&m.ID, &m.VariantID, &m.Quantity); err != nil {
			return err
		}
		movements = append(movements, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(movements) == 0 {
		return errors.New("no in-transit stock to receive")
	}

	// 3️⃣ For each movement: upsert stock into branch warehouse + mark RECEIVED
	for _, m := range movements {
		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1, $2, $3, 'FINISHED_GOOD', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity,
			             updated_at = NOW()
		`, m.VariantID, toWarehouseID, m.Quantity)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE stock_movements
			SET status = 'RECEIVED', updated_at = NOW()
			WHERE id = $1
		`, m.ID)
		if err != nil {
			return err
		}
	}

	// 4️⃣ Check if all movements are now received
	var inTransitCount int
	err = tx.QueryRow(`
		SELECT COUNT(*)
		FROM stock_movements
		WHERE stock_request_id = $1
		AND status = 'IN_TRANSIT'
	`, requestID).Scan(&inTransitCount)
	if err != nil {
		return err
	}

	// If no more in-transit and fully dispatched → COMPLETED
	if inTransitCount == 0 && status == "FULL_DISPATCH" {
		_, err = tx.Exec(`
			UPDATE stock_requests
			SET status = 'COMPLETED', updated_at = NOW()
			WHERE id = $1
		`, requestID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

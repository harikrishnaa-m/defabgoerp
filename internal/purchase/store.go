package purchase

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CREATE PO
func (s *Store) Create(in CreatePurchaseOrderInput) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	poID := uuid.New().String()
	poNumber := "PO-" + time.Now().Format("20060102150405")

	_, err = tx.Exec(`
	INSERT INTO purchase_orders
	(id, po_number, supplier_id, warehouse_id, status, expected_date, created_at)
	VALUES ($1,$2,$3,$4,'DRAFT',$5,NOW())
	`,
		poID,
		poNumber,
		in.SupplierID,
		in.WarehouseID,
		in.ExpectedDate,
	)
	if err != nil {
		return "", err
	}

	var totalAmount, taxAmount float64

	for _, item := range in.Items {
		gstAmount := item.Quantity * item.UnitPrice * item.GSTPercent / 100
		totalPrice := (item.Quantity * item.UnitPrice) + gstAmount
		totalAmount += item.Quantity * item.UnitPrice
		taxAmount += gstAmount

		_, err := tx.Exec(`
		INSERT INTO purchase_order_items
		(id, purchase_order_id, item_name, description, hsn_code, unit,
		 quantity, unit_price, gst_percent, gst_amount, total_price)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`,
			uuid.New().String(),
			poID,
			item.ItemName,
			item.Description,
			item.HSNCode,
			item.Unit,
			item.Quantity,
			item.UnitPrice,
			item.GSTPercent,
			gstAmount,
			totalPrice,
		)
		if err != nil {
			return "", err
		}
	}

	grandTotal := totalAmount + taxAmount
	_, err = tx.Exec(`
		UPDATE purchase_orders
		SET total_amount = $1, tax_amount = $2, grand_total = $3
		WHERE id = $4
	`, totalAmount, taxAmount, grandTotal, poID)
	if err != nil {
		return "", err
	}

	return poID, tx.Commit()
}

// LIST
func (s *Store) List(limit, offset int) ([]POListRow, error) {
	rows, err := s.db.Query(`
	SELECT po.id, po.po_number, po.status,
	       s.name AS supplier_name,
	       po.grand_total, po.created_at::text
	FROM purchase_orders po
	LEFT JOIN suppliers s ON s.id = po.supplier_id
	ORDER BY po.created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []POListRow
	for rows.Next() {
		var r POListRow
		if err := rows.Scan(&r.ID, &r.PONumber, &r.Status, &r.SupplierName, &r.GrandTotal, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// GET with items
func (s *Store) Get(id string) (*PODetailResponse, error) {
	var po PODetailResponse
	err := s.db.QueryRow(`
	SELECT id, po_number, supplier_id, warehouse_id, status,
	       expected_date::text, total_amount, tax_amount, grand_total, created_at::text
	FROM purchase_orders
	WHERE id = $1
	`, id).Scan(
		&po.ID, &po.PONumber, &po.SupplierID, &po.WarehouseID, &po.Status,
		&po.ExpectedDate, &po.TotalAmount, &po.TaxAmount, &po.GrandTotal, &po.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
	SELECT id, item_name, description, hsn_code, unit,
	       quantity, unit_price, gst_percent, gst_amount, total_price, received_qty
	FROM purchase_order_items
	WHERE purchase_order_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item POItemResponse
		if err := rows.Scan(
			&item.ID, &item.ItemName, &item.Description, &item.HSNCode, &item.Unit,
			&item.Quantity, &item.UnitPrice, &item.GSTPercent, &item.GSTAmount,
			&item.TotalPrice, &item.ReceivedQty,
		); err != nil {
			return nil, err
		}
		po.Items = append(po.Items, item)
	}

	if po.Items == nil {
		po.Items = []POItemResponse{}
	}

	return &po, nil
}

// UPDATE STATUS
func (s *Store) UpdateStatus(id, status string) error {
	valid := map[string]bool{
		"DRAFT": true, "CONFIRMED": true, "PARTIAL": true,
		"RECEIVED": true, "CANCELLED": true,
	}
	if !valid[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	_, err := s.db.Exec(`
	UPDATE purchase_orders
	SET status=$1
	WHERE id=$2
	`, status, id)

	return err
}

// recalcTotals recalculates total_amount, tax_amount, grand_total from items.
func (s *Store) recalcTotals(tx *sql.Tx, poID string) error {
	_, err := tx.Exec(`
		UPDATE purchase_orders
		SET total_amount = COALESCE((SELECT SUM(quantity * unit_price) FROM purchase_order_items WHERE purchase_order_id = $1), 0),
		    tax_amount   = COALESCE((SELECT SUM(gst_amount) FROM purchase_order_items WHERE purchase_order_id = $1), 0),
		    grand_total  = COALESCE((SELECT SUM(total_price) FROM purchase_order_items WHERE purchase_order_id = $1), 0)
		WHERE id = $1
	`, poID)
	return err
}

// AddItem adds one item to an existing PO and recalculates totals.
func (s *Store) AddItem(poID string, in AddPOItemInput) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	gstAmount := in.Quantity * in.UnitPrice * in.GSTPercent / 100
	totalPrice := (in.Quantity * in.UnitPrice) + gstAmount
	itemID := uuid.New().String()

	_, err = tx.Exec(`
		INSERT INTO purchase_order_items
		(id, purchase_order_id, item_name, description, hsn_code, unit,
		 quantity, unit_price, gst_percent, gst_amount, total_price)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, itemID, poID, in.ItemName, in.Description, in.HSNCode, in.Unit,
		in.Quantity, in.UnitPrice, in.GSTPercent, gstAmount, totalPrice)
	if err != nil {
		return "", err
	}

	if err := s.recalcTotals(tx, poID); err != nil {
		return "", err
	}

	return itemID, tx.Commit()
}

// UpdateItem updates a PO item and recalculates totals.
func (s *Store) UpdateItem(poID, itemID string, in UpdatePOItemInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Fetch current values
	var curName, curDesc, curHSN, curUnit string
	var curQty, curPrice, curGST float64
	err = tx.QueryRow(`
		SELECT item_name, description, hsn_code, unit, quantity, unit_price, gst_percent
		FROM purchase_order_items
		WHERE id = $1 AND purchase_order_id = $2
	`, itemID, poID).Scan(&curName, &curDesc, &curHSN, &curUnit, &curQty, &curPrice, &curGST)
	if err != nil {
		return err
	}

	// Apply patches
	if in.ItemName != nil {
		curName = *in.ItemName
	}
	if in.Description != nil {
		curDesc = *in.Description
	}
	if in.HSNCode != nil {
		curHSN = *in.HSNCode
	}
	if in.Unit != nil {
		curUnit = *in.Unit
	}
	if in.Quantity != nil {
		curQty = *in.Quantity
	}
	if in.UnitPrice != nil {
		curPrice = *in.UnitPrice
	}
	if in.GSTPercent != nil {
		curGST = *in.GSTPercent
	}

	gstAmount := curQty * curPrice * curGST / 100
	totalPrice := (curQty * curPrice) + gstAmount

	_, err = tx.Exec(`
		UPDATE purchase_order_items
		SET item_name = $1, description = $2, hsn_code = $3, unit = $4,
		    quantity = $5, unit_price = $6, gst_percent = $7,
		    gst_amount = $8, total_price = $9
		WHERE id = $10 AND purchase_order_id = $11
	`, curName, curDesc, curHSN, curUnit, curQty, curPrice, curGST, gstAmount, totalPrice, itemID, poID)
	if err != nil {
		return err
	}

	if err := s.recalcTotals(tx, poID); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteItem removes a PO item and recalculates totals.
func (s *Store) DeleteItem(poID, itemID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		DELETE FROM purchase_order_items
		WHERE id = $1 AND purchase_order_id = $2
	`, itemID, poID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found")
	}

	if err := s.recalcTotals(tx, poID); err != nil {
		return err
	}

	return tx.Commit()
}

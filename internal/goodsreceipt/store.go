package goodsreceipt

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

// generateGRNNumber creates the next GRN-XXX number.
func (s *Store) generateGRNNumber() (string, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM goods_receipts`).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("GRN-%03d", count+1), nil
}

// Create creates a goods receipt, upserts stock, records stock movements,
// and updates received_qty on the purchase order items.
func (s *Store) Create(in CreateGoodsReceiptInput, userID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if len(in.Items) == 0 {
		return "", errors.New("at least one item required")
	}

	grnNumber, err := s.generateGRNNumber()
	if err != nil {
		return "", err
	}

	// Insert goods_receipts row
	var grnID string
	err = tx.QueryRow(`
		INSERT INTO goods_receipts
			(grn_number, purchase_order_id, supplier_id, warehouse_id, received_by, received_date, reference, status)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, 'COMPLETED')
		RETURNING id
	`, grnNumber, in.PurchaseOrderID, in.SupplierID, in.WarehouseID, userID, in.Reference).Scan(&grnID)
	if err != nil {
		return "", fmt.Errorf("insert goods_receipts: %w", err)
	}

	for _, item := range in.Items {
		if item.ReceivedQty <= 0 {
			return "", errors.New("received_qty must be greater than zero")
		}

		// Insert goods_receipt_items row
		_, err := tx.Exec(`
			INSERT INTO goods_receipt_items
				(goods_receipt_id, purchase_order_item_id, ordered_qty, received_qty)
			VALUES ($1, $2, $3, $4)
		`, grnID, item.PurchaseOrderItemID, item.OrderedQty, item.ReceivedQty)
		if err != nil {
			return "", fmt.Errorf("insert goods_receipt_items: %w", err)
		}

		// Update received_qty on the purchase order item
		_, err = tx.Exec(`
			UPDATE purchase_order_items
			SET received_qty = received_qty + $1
			WHERE id = $2
		`, item.ReceivedQty, item.PurchaseOrderItemID)
		if err != nil {
			return "", fmt.Errorf("update received_qty: %w", err)
		}
	}

	// Update PO status based on received quantities
	_, err = tx.Exec(`
		UPDATE purchase_orders
		SET status = CASE
			WHEN (SELECT COUNT(*) FROM purchase_order_items WHERE purchase_order_id = $1 AND received_qty < quantity) = 0
				THEN 'RECEIVED'
			WHEN (SELECT SUM(received_qty) FROM purchase_order_items WHERE purchase_order_id = $1) > 0
				THEN 'PARTIAL'
			ELSE status
		END
		WHERE id = $1
	`, in.PurchaseOrderID)
	if err != nil {
		return "", fmt.Errorf("update po status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return grnID, nil
}

// GetByID returns a goods receipt with its items.
func (s *Store) GetByID(id string) (*GoodsReceiptResponse, error) {
	var grn GoodsReceiptResponse
	err := s.db.QueryRow(`
		SELECT id, grn_number, purchase_order_id, supplier_id, warehouse_id,
		       received_by, received_date::text, reference, status, created_at::text
		FROM goods_receipts
		WHERE id = $1
	`, id).Scan(
		&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.SupplierID,
		&grn.WarehouseID, &grn.ReceivedBy, &grn.ReceivedDate, &grn.Reference,
		&grn.Status, &grn.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT gri.id, gri.purchase_order_item_id, poi.item_name, gri.ordered_qty, gri.received_qty
		FROM goods_receipt_items gri
		JOIN purchase_order_items poi ON poi.id = gri.purchase_order_item_id
		WHERE gri.goods_receipt_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item GoodsReceiptItemResponse
		if err := rows.Scan(&item.ID, &item.PurchaseOrderItemID, &item.ItemName, &item.OrderedQty, &item.ReceivedQty); err != nil {
			return nil, err
		}
		grn.Items = append(grn.Items, item)
	}

	return &grn, nil
}

// ListByPO returns all goods receipts for a purchase order.
func (s *Store) ListByPO(poID string) ([]GoodsReceiptResponse, error) {
	rows, err := s.db.Query(`
		SELECT id, grn_number, purchase_order_id, supplier_id, warehouse_id,
		       received_by, received_date::text, reference, status, created_at::text
		FROM goods_receipts
		WHERE purchase_order_id = $1
		ORDER BY created_at DESC
	`, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GoodsReceiptResponse
	for rows.Next() {
		var grn GoodsReceiptResponse
		if err := rows.Scan(
			&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.SupplierID,
			&grn.WarehouseID, &grn.ReceivedBy, &grn.ReceivedDate, &grn.Reference,
			&grn.Status, &grn.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, grn)
	}

	return results, nil
}

// List returns all goods receipts, newest first.
func (s *Store) List() ([]GoodsReceiptResponse, error) {
	rows, err := s.db.Query(`
		SELECT id, grn_number, purchase_order_id, supplier_id, warehouse_id,
		       received_by, received_date::text, reference, status, created_at::text
		FROM goods_receipts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GoodsReceiptResponse
	for rows.Next() {
		var grn GoodsReceiptResponse
		if err := rows.Scan(
			&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.SupplierID,
			&grn.WarehouseID, &grn.ReceivedBy, &grn.ReceivedDate, &grn.Reference,
			&grn.Status, &grn.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, grn)
	}

	return results, nil
}

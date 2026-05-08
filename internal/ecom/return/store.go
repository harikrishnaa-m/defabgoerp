package ecomreturn

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

// ─── Data models ──────────────────────────────────────────────────────────────

type ReturnRequest struct {
	ID                  string    `json:"id"`
	OrderID             string    `json:"order_id"`
	OrderNumber         string    `json:"order_number"`
	CustomerID          string    `json:"customer_id"`
	Reason              string    `json:"reason"`
	Status              string    `json:"status"`
	PayoutMethod        *string   `json:"payout_method,omitempty"`
	PayoutUPI           *string   `json:"payout_upi,omitempty"`
	PayoutAccountNumber *string   `json:"payout_account_number,omitempty"`
	PayoutIFSC          *string   `json:"payout_ifsc,omitempty"`
	PayoutAccountName   *string   `json:"payout_account_name,omitempty"`
	PayoutTransferID    *string   `json:"payout_transfer_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type OrderReturnInfo struct {
	OrderID       string
	OrderNumber   string
	CustomerID    string
	Status        string
	PaymentMethod string
	PaymentStatus string
	GrandTotal    float64
	DeliveredAt   *time.Time
}

// ─── Customer operations ──────────────────────────────────────────────────────

// RequestReturn validates eligibility and creates a return request.
func (s *Store) RequestReturn(customerID, orderID, reason string, payoutMethod, payoutUPI, payoutAccountNumber, payoutIFSC, payoutAccountName *string) (*ReturnRequest, error) {
	// Fetch order details
	var info OrderReturnInfo
	err := s.db.QueryRow(`
		SELECT id, order_number, customer_id, status, payment_method, payment_status, grand_total, delivered_at
		FROM ecom_orders
		WHERE id = $1 AND customer_id = $2
	`, orderID, customerID).Scan(
		&info.OrderID, &info.OrderNumber, &info.CustomerID,
		&info.Status, &info.PaymentMethod, &info.PaymentStatus,
		&info.GrandTotal, &info.DeliveredAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order not found")
	}
	if err != nil {
		return nil, err
	}

	// Only DELIVERED orders can be returned
	if info.Status != "DELIVERED" {
		return nil, fmt.Errorf("only delivered orders are eligible for return")
	}

	// 7-day return window
	if info.DeliveredAt == nil {
		return nil, fmt.Errorf("delivery date not recorded; cannot process return")
	}
	if time.Since(*info.DeliveredAt) > 7*24*time.Hour {
		return nil, fmt.Errorf("return window has expired (7 days from delivery)")
	}

	// Check no existing return request
	var existingCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM ecom_returns WHERE order_id = $1`, orderID).Scan(&existingCount)
	if existingCount > 0 {
		return nil, fmt.Errorf("a return request already exists for this order")
	}

	// COD orders require payout details
	if info.PaymentMethod == "COD" {
		if payoutMethod == nil {
			return nil, fmt.Errorf("payout_method is required for COD orders (UPI or BANK)")
		}
		if *payoutMethod == "UPI" && (payoutUPI == nil || *payoutUPI == "") {
			return nil, fmt.Errorf("payout_upi is required for UPI payout")
		}
		if *payoutMethod == "BANK" && (payoutAccountNumber == nil || payoutIFSC == nil || payoutAccountName == nil) {
			return nil, fmt.Errorf("payout_account_number, payout_ifsc, and payout_account_name are required for BANK payout")
		}
	}

	var ret ReturnRequest
	err = s.db.QueryRow(`
		INSERT INTO ecom_returns (
			order_id, customer_id, reason,
			payout_method, payout_upi, payout_account_number,
			payout_ifsc, payout_account_name
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, order_id, customer_id, reason, status,
		          payout_method, payout_upi, payout_account_number,
		          payout_ifsc, payout_account_name, payout_transfer_id,
		          created_at, updated_at
	`, orderID, customerID, reason, payoutMethod, payoutUPI, payoutAccountNumber, payoutIFSC, payoutAccountName).Scan(
		&ret.ID, &ret.OrderID, &ret.CustomerID, &ret.Reason, &ret.Status,
		&ret.PayoutMethod, &ret.PayoutUPI, &ret.PayoutAccountNumber,
		&ret.PayoutIFSC, &ret.PayoutAccountName, &ret.PayoutTransferID,
		&ret.CreatedAt, &ret.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	ret.OrderNumber = info.OrderNumber
	return &ret, nil
}

// GetReturn returns a return request for a specific customer.
func (s *Store) GetReturn(customerID, returnID string) (*ReturnRequest, error) {
	var ret ReturnRequest
	err := s.db.QueryRow(`
		SELECT r.id, r.order_id, o.order_number, r.customer_id, r.reason, r.status,
		       r.payout_method, r.payout_upi, r.payout_account_number,
		       r.payout_ifsc, r.payout_account_name, r.payout_transfer_id,
		       r.created_at, r.updated_at
		FROM ecom_returns r
		JOIN ecom_orders o ON o.id = r.order_id
		WHERE r.id = $1 AND r.customer_id = $2
	`, returnID, customerID).Scan(
		&ret.ID, &ret.OrderID, &ret.OrderNumber, &ret.CustomerID,
		&ret.Reason, &ret.Status,
		&ret.PayoutMethod, &ret.PayoutUPI, &ret.PayoutAccountNumber,
		&ret.PayoutIFSC, &ret.PayoutAccountName, &ret.PayoutTransferID,
		&ret.CreatedAt, &ret.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("return request not found")
	}
	return &ret, err
}

// ListReturns returns all return requests for a customer.
func (s *Store) ListReturns(customerID string) ([]ReturnRequest, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.order_id, o.order_number, r.customer_id, r.reason, r.status,
		       r.payout_method, r.payout_upi, r.payout_account_number,
		       r.payout_ifsc, r.payout_account_name, r.payout_transfer_id,
		       r.created_at, r.updated_at
		FROM ecom_returns r
		JOIN ecom_orders o ON o.id = r.order_id
		WHERE r.customer_id = $1
		ORDER BY r.created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReturnRequest
	for rows.Next() {
		var ret ReturnRequest
		if err := rows.Scan(
			&ret.ID, &ret.OrderID, &ret.OrderNumber, &ret.CustomerID,
			&ret.Reason, &ret.Status,
			&ret.PayoutMethod, &ret.PayoutUPI, &ret.PayoutAccountNumber,
			&ret.PayoutIFSC, &ret.PayoutAccountName, &ret.PayoutTransferID,
			&ret.CreatedAt, &ret.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, ret)
	}
	return list, nil
}

// ─── Admin operations ─────────────────────────────────────────────────────────

// AdminListReturns returns all return requests with optional status filter.
func (s *Store) AdminListReturns(status string, page, limit int) ([]ReturnRequest, error) {
	offset := (page - 1) * limit
	query := `
		SELECT r.id, r.order_id, o.order_number, r.customer_id, r.reason, r.status,
		       r.payout_method, r.payout_upi, r.payout_account_number,
		       r.payout_ifsc, r.payout_account_name, r.payout_transfer_id,
		       r.created_at, r.updated_at
		FROM ecom_returns r
		JOIN ecom_orders o ON o.id = r.order_id
	`
	args := []interface{}{}
	if status != "" {
		query += " WHERE r.status = $1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3"
		args = append(args, status, limit, offset)
	} else {
		query += " ORDER BY r.created_at DESC LIMIT $1 OFFSET $2"
		args = append(args, limit, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReturnRequest
	for rows.Next() {
		var ret ReturnRequest
		if err := rows.Scan(
			&ret.ID, &ret.OrderID, &ret.OrderNumber, &ret.CustomerID,
			&ret.Reason, &ret.Status,
			&ret.PayoutMethod, &ret.PayoutUPI, &ret.PayoutAccountNumber,
			&ret.PayoutIFSC, &ret.PayoutAccountName, &ret.PayoutTransferID,
			&ret.CreatedAt, &ret.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, ret)
	}
	return list, nil
}

// AdminGetReturnWithOrder returns return + order info for processing.
func (s *Store) AdminGetReturnWithOrder(returnID string) (*ReturnRequest, *OrderReturnInfo, error) {
	var ret ReturnRequest
	var info OrderReturnInfo
	err := s.db.QueryRow(`
		SELECT r.id, r.order_id, o.order_number, r.customer_id, r.reason, r.status,
		       r.payout_method, r.payout_upi, r.payout_account_number,
		       r.payout_ifsc, r.payout_account_name, r.payout_transfer_id,
		       r.created_at, r.updated_at,
		       o.customer_id, o.status, o.payment_method, o.payment_status, o.grand_total
		FROM ecom_returns r
		JOIN ecom_orders o ON o.id = r.order_id
		WHERE r.id = $1
	`, returnID).Scan(
		&ret.ID, &ret.OrderID, &ret.OrderNumber, &ret.CustomerID,
		&ret.Reason, &ret.Status,
		&ret.PayoutMethod, &ret.PayoutUPI, &ret.PayoutAccountNumber,
		&ret.PayoutIFSC, &ret.PayoutAccountName, &ret.PayoutTransferID,
		&ret.CreatedAt, &ret.UpdatedAt,
		&info.CustomerID, &info.Status, &info.PaymentMethod,
		&info.PaymentStatus, &info.GrandTotal,
	)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("return request not found")
	}
	if err != nil {
		return nil, nil, err
	}
	info.OrderID = ret.OrderID
	info.OrderNumber = ret.OrderNumber
	return &ret, &info, nil
}

// AdminApproveReturn sets status to APPROVED.
func (s *Store) AdminApproveReturn(returnID string) error {
	res, err := s.db.Exec(`
		UPDATE ecom_returns SET status = 'APPROVED', updated_at = NOW()
		WHERE id = $1 AND status = 'REQUESTED'
	`, returnID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("return not found or not in REQUESTED state")
	}
	return nil
}

// AdminRejectReturn sets status to REJECTED.
func (s *Store) AdminRejectReturn(returnID string) error {
	res, err := s.db.Exec(`
		UPDATE ecom_returns SET status = 'REJECTED', updated_at = NOW()
		WHERE id = $1 AND status = 'REQUESTED'
	`, returnID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("return not found or not in REQUESTED state")
	}
	return nil
}

// MarkRefundInitiated sets status to REFUND_INITIATED and updates the order.
func (s *Store) MarkRefundInitiated(returnID, orderNumber string) error {
	_, err := s.db.Exec(`UPDATE ecom_returns SET status = 'REFUND_INITIATED', updated_at = NOW() WHERE id = $1`, returnID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE ecom_orders SET payment_status = 'REFUND_INITIATED', updated_at = NOW() WHERE order_number = $1`, orderNumber)
	return err
}

// MarkPayoutInitiated sets status to PAYOUT_INITIATED and records the transfer ID.
func (s *Store) MarkPayoutInitiated(returnID, transferID, orderNumber string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_returns SET status = 'PAYOUT_INITIATED', payout_transfer_id = $2, updated_at = NOW()
		WHERE id = $1
	`, returnID, transferID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE ecom_orders SET payment_status = 'PAYOUT_INITIATED', updated_at = NOW() WHERE order_number = $1`, orderNumber)
	return err
}

// MarkPayoutCompleted sets status to PAYOUT_COMPLETED by transfer ID and marks the order as RETURNED.
func (s *Store) MarkPayoutCompleted(transferID string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_returns SET status = 'PAYOUT_COMPLETED', updated_at = NOW()
		WHERE payout_transfer_id = $1
	`, transferID)
	if err != nil {
		return err
	}
	// Update the linked order's payment_status and status too
	_, err = s.db.Exec(`
		UPDATE ecom_orders
		SET payment_status = 'PAYOUT_COMPLETED', status = 'RETURNED', updated_at = NOW()
		WHERE id = (
			SELECT order_id FROM ecom_returns WHERE payout_transfer_id = $1 LIMIT 1
		)
	`, transferID)
	return err
}

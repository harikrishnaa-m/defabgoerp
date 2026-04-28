package payment

import (
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveCashfreeOrder saves the cf_order_id and payment_session_id to an existing order.
func (s *Store) SaveCashfreeOrder(orderID, cfOrderID, sessionID string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_orders
		SET cf_order_id = $2, payment_session_id = $3, updated_at = NOW()
		WHERE id = $1
	`, orderID, cfOrderID, sessionID)
	return err
}

// MarkOrderPaid updates payment_status to PAID. orderNumber is our ECOM-XXXXX order number
// (echoed back by Cashfree in webhook data.order.order_id).
func (s *Store) MarkOrderPaid(orderNumber, paymentRef string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_orders
		SET payment_status = 'PAID', payment_ref = $2, updated_at = NOW()
		WHERE order_number = $1
	`, orderNumber, paymentRef)
	return err
}

// GetOrderForPayment returns order details needed to create Cashfree session.
func (s *Store) GetOrderForPayment(orderID string) (cfOrderID, sessionID, status string, grandTotal float64, err error) {
	err = s.db.QueryRow(`
		SELECT COALESCE(cf_order_id, ''), COALESCE(payment_session_id, ''),
		       payment_status, grand_total
		FROM ecom_orders WHERE id = $1
	`, orderID).Scan(&cfOrderID, &sessionID, &status, &grandTotal)
	return
}

// MarkOrderPaidByCFOrderID marks an order as PAID using the Cashfree order ID (no payment ref).
func (s *Store) MarkOrderPaidByCFOrderID(cfOrderID string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_orders
		SET payment_status = 'PAID', updated_at = NOW()
		WHERE cf_order_id = $1
	`, cfOrderID)
	return err
}

// MarkOrderPaidByOrderNumber marks an order as PAID using our order_number.
func (s *Store) MarkOrderPaidByOrderNumber(orderNumber string) error {
	_, err := s.db.Exec(`
		UPDATE ecom_orders
		SET payment_status = 'PAID', updated_at = NOW()
		WHERE order_number = $1
	`, orderNumber)
	return err
}

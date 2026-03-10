package purchase

import (
	"database/sql"
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

	for _, item := range in.Items {
		_, err := tx.Exec(`
		INSERT INTO purchase_order_items
		(id, purchase_order_id, variant_id, quantity)
		VALUES ($1,$2,$3,$4)
		`,
			uuid.New().String(),
			poID,
			item.VariantID,
			item.Quantity,
		)
		if err != nil {
			return "", err
		}
	}

	return poID, tx.Commit()
}

// LIST
func (s *Store) List(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT id, po_number, status, created_at
	FROM purchase_orders
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
}

// GET
func (s *Store) Get(id string) *sql.Row {
	return s.db.QueryRow(`
	SELECT id, po_number, supplier_id, warehouse_id, status, expected_date, created_at
	FROM purchase_orders
	WHERE id=$1
	`, id)
}

// UPDATE STATUS
func (s *Store) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`
	UPDATE purchase_orders
	SET status=$1
	WHERE id=$2
	`, status, id)

	return err
}

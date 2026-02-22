package goodsreceipt

import (
	"database/sql"
	"errors"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(in CreateGoodsReceiptInput, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return errors.New("quantity must be greater than zero")
		}

		// 1️⃣ UPSERT stock
		_, err := tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity)
			VALUES ($1,$2,$3)
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET
				quantity = stocks.quantity + EXCLUDED.quantity,
				updated_at = NOW()
		`,
			item.VariantID,
			in.WarehouseID,
			item.Quantity,
		)
		if err != nil {
			return err
		}

		// 2️⃣ Record stock movement (IN)
		_, err = tx.Exec(`
			INSERT INTO stock_movements
			(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1,$2,$3,'IN',$4,'COMPLETED')
		`,
			item.VariantID,
			in.WarehouseID,
			item.Quantity,
			in.Reference,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

package stocktransfer

import (
	"database/sql"
	"errors"
	"github.com/shopspring/decimal"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(in CreateStockTransferInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return errors.New("quantity must be greater than zero")
		}

		// 1️⃣ Check available stock (LOCK ROW)
		var available int
		err := tx.QueryRow(`
			SELECT quantity
			FROM stocks
			WHERE variant_id=$1 AND warehouse_id=$2
			FOR UPDATE
		`,
			item.VariantID,
			in.FromWarehouseID,
		).Scan(&available)

		if err == sql.ErrNoRows {
			return errors.New("stock not found in source warehouse")
		}
		if err != nil {
			return err
		}

		if available < item.Quantity {
			return errors.New("insufficient stock")
		}

		// 2️⃣ Deduct from source warehouse
		_, err = tx.Exec(`
			UPDATE stocks
			SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id=$2 AND warehouse_id=$3
		`,
			item.Quantity,
			item.VariantID,
			in.FromWarehouseID,
		)
		if err != nil {
			return err
		}

		// 3️⃣ Add to destination warehouse
		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity)
			VALUES ($1,$2,$3)
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET
				quantity = stocks.quantity + EXCLUDED.quantity,
				updated_at = NOW()
		`,
			item.VariantID,
			in.ToWarehouseID,
			item.Quantity,
		)
		if err != nil {
			return err
		}

		// 4️⃣ Record movement
		_, err = tx.Exec(`
			INSERT INTO stock_movements
			(variant_id, from_warehouse_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1,$2,$3,$4,'TRANSFER',$5,'COMPLETED')
		`,
			item.VariantID,
			in.FromWarehouseID,
			in.ToWarehouseID,
			item.Quantity,
			in.Reference,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}


func (s *Store) ReceiveTransfer(
	movementID string,
	receivedQty decimal.Decimal,
	remarks string,
) error {

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		variantID string
		toWH      sql.NullString
		status    string
	)

	err = tx.QueryRow(`
		SELECT variant_id, to_warehouse_id, status
		FROM stock_movements
		WHERE id = $1
		FOR UPDATE
	`, movementID).Scan(&variantID, &toWH, &status)

	if err != nil {
		return err
	}

	if status != "IN_TRANSIT" {
		return errors.New("movement not in transit")
	}

	if !toWH.Valid {
		return errors.New("destination warehouse missing")
	}

	// Increase stock in destination
	_, err = tx.Exec(`
		INSERT INTO stocks (variant_id, warehouse_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (variant_id, warehouse_id)
		DO UPDATE SET
		  quantity = stocks.quantity + EXCLUDED.quantity,
		  updated_at = NOW()
	`, variantID, toWH.String, receivedQty)

	if err != nil {
		return err
	}

	// Mark movement completed
	_, err = tx.Exec(`
		UPDATE stock_movements
		SET status = 'COMPLETED',
		    updated_at = NOW()
		WHERE id = $1
	`, movementID)

	if err != nil {
		return err
	}

	return tx.Commit()
}

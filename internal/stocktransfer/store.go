package stocktransfer

import (
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"
)

type Store struct {
	db *sql.DB
}

type BranchVariantStock struct {
	VariantID   string          `json:"variant_id"`
	VariantCode int             `json:"variant_code"`
	VariantName string          `json:"variant_name"`
	WarehouseID string          `json:"warehouse_id"`
	Warehouse   string          `json:"warehouse_name"`
	Quantity    decimal.Decimal `json:"quantity"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetBranchVariantStock(branchID, variantCode string) ([]BranchVariantStock, error) {
	rows, err := s.db.Query(`
		WITH branch_warehouse AS (
			SELECT id, name
			FROM warehouses
			WHERE branch_id = $2
			ORDER BY created_at, id
			LIMIT 1
		)
		SELECT
			v.id,
			v.variant_code,
			v.name,
			w.id,
			w.name,
			COALESCE(s.quantity, 0)
		FROM variants v
		CROSS JOIN branch_warehouse w
		LEFT JOIN stocks s ON s.variant_id = v.id AND s.warehouse_id = w.id
		WHERE v.variant_code::text = $1
		  AND v.is_active = true
		ORDER BY v.id
	`, variantCode, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stock := make([]BranchVariantStock, 0)
	for rows.Next() {
		var item BranchVariantStock
		if err := rows.Scan(
			&item.VariantID,
			&item.VariantCode,
			&item.VariantName,
			&item.WarehouseID,
			&item.Warehouse,
			&item.Quantity,
		); err != nil {
			return nil, err
		}
		stock = append(stock, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(stock) > 0 {
		return stock, nil
	}

	var variantExists bool
	err = s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM variants
			WHERE variant_code::text = $1 AND is_active = true
		)
	`, variantCode).Scan(&variantExists)
	if err != nil {
		return nil, err
	}
	if !variantExists {
		return nil, errors.New("variant not found")
	}
	return nil, errors.New("warehouse not found for branch")
}

func (s *Store) CreateInterWarehouseTransfer(in InterWarehouseTransferInput, managerBranchID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var fromBranchID, toBranchID sql.NullString
	err = tx.QueryRow(`
		SELECT source.branch_id, destination.branch_id
		FROM warehouses source
		LEFT JOIN warehouses destination ON destination.id = $2
		WHERE source.id = $1
		FOR UPDATE OF source
	`, in.FromWarehouseID, in.ToWarehouseID).Scan(&fromBranchID, &toBranchID)
	if err == sql.ErrNoRows || !fromBranchID.Valid || !toBranchID.Valid {
		return errors.New("source or destination warehouse not found")
	}
	if err != nil {
		return err
	}
	if managerBranchID != "" && fromBranchID.String != managerBranchID {
		return errors.New("source warehouse is not associated with your branch")
	}
	if fromBranchID.String == toBranchID.String {
		return errors.New("source and destination warehouses must belong to different branches")
	}

	for _, item := range in.Items {
		var available decimal.Decimal
		err = tx.QueryRow(`
			SELECT quantity
			FROM stocks
			WHERE variant_id = $1 AND warehouse_id = $2
			FOR UPDATE
		`, item.VariantID, in.FromWarehouseID).Scan(&available)
		if err == sql.ErrNoRows {
			return errors.New("stock not found in source warehouse")
		}
		if err != nil {
			return err
		}
		if available.LessThan(item.Quantity) {
			return errors.New("insufficient stock")
		}

		_, err = tx.Exec(`
			UPDATE stocks
			SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id = $2 AND warehouse_id = $3
		`, item.Quantity, item.VariantID, in.FromWarehouseID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity)
			VALUES ($1, $2, $3)
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET
				quantity = stocks.quantity + EXCLUDED.quantity,
				updated_at = NOW()
		`, item.VariantID, in.ToWarehouseID, item.Quantity)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, from_warehouse_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, $4, 'TRANSFER', NULLIF($5, ''), 'COMPLETED')
		`, item.VariantID, in.FromWarehouseID, in.ToWarehouseID, item.Quantity, in.Reason)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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

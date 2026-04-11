package production

import (
	"database/sql"
	"fmt"
	"strings"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ──────────────────────────────────────────
// Create
// ──────────────────────────────────────────

func (s *Store) CreateProductionOrder(in CreateProductionOrderInput, userID, branchID, warehouseID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// ── Resolve output_variant_id based on scenario ──

	variantID := in.OutputVariantID
	productID := in.OutputProductID

	// Scenario 3: New product + new variant
	if in.NewProduct != nil && in.NewVariant != nil {
		uom := in.NewProduct.UOM
		if uom == "" {
			uom = "Unit"
		}
		err = tx.QueryRow(`
			INSERT INTO products (name, category_id, brand, description, fabric_composition, pattern, occasion, care_instructions, uom)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id
		`, in.NewProduct.Name, nilIfEmpty(in.NewProduct.CategoryID), in.NewProduct.Brand,
			in.NewProduct.Description, in.NewProduct.FabricComposition, in.NewProduct.Pattern,
			in.NewProduct.Occasion, in.NewProduct.CareInstructions, uom).Scan(&productID)
		if err != nil {
			return "", fmt.Errorf("create product: %w", err)
		}

		// Increment category product count
		if in.NewProduct.CategoryID != "" {
			_, err = tx.Exec(`UPDATE categories SET products_count = products_count + 1 WHERE id = $1`, in.NewProduct.CategoryID)
			if err != nil {
				return "", fmt.Errorf("update category count: %w", err)
			}
		}
	}

	// Scenario 2 or 3: Create new variant under existing or newly created product
	if in.NewVariant != nil && productID != "" {
		// Auto-generate SKU
		var brand, productName string
		err = tx.QueryRow(`SELECT COALESCE(brand,''), name FROM products WHERE id = $1`, productID).Scan(&brand, &productName)
		if err != nil {
			return "", fmt.Errorf("product lookup for SKU: %w", err)
		}

		sku := strings.ToUpper(first3(brand) + " " + first3(productName))
		for _, avid := range in.NewVariant.AttributeValueIDs {
			var val string
			if tx.QueryRow(`SELECT value FROM attribute_values WHERE id = $1`, avid).Scan(&val) == nil {
				sku += " " + strings.ToUpper(first3(val))
			}
		}
		baseSku := sku
		var exists bool
		for i := 1; ; i++ {
			tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM variants WHERE sku = $1)`, sku).Scan(&exists)
			if !exists {
				break
			}
			sku = fmt.Sprintf("%s %d", baseSku, i)
		}

		err = tx.QueryRow(`
			INSERT INTO variants (product_id, name, sku, price, cost_price, barcode)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id
		`, productID, in.NewVariant.Name, sku, in.NewVariant.Price, in.NewVariant.CostPrice, sku).Scan(&variantID)
		if err != nil {
			return "", fmt.Errorf("create variant: %w", err)
		}

		// Attribute mappings
		for _, avid := range in.NewVariant.AttributeValueIDs {
			_, err = tx.Exec(`INSERT INTO variant_attribute_mapping (variant_id, attribute_value_id) VALUES ($1,$2)`, variantID, avid)
			if err != nil {
				return "", fmt.Errorf("map attribute: %w", err)
			}
		}
	}

	if variantID == "" {
		return "", fmt.Errorf("could not resolve output variant")
	}

	// ── Create the production order ──

	prodNum := s.nextProdNumber(tx)

	var branchParam interface{}
	if branchID != "" {
		branchParam = branchID
	}

	var prodID string
	err = tx.QueryRow(`
		INSERT INTO production_orders
			(production_number, branch_id, warehouse_id, output_variant_id, output_quantity,
			 status, notes, created_by)
		VALUES ($1,$2,$3,$4,$5,'PLANNED',$6,$7)
		RETURNING id
	`, prodNum, branchParam, warehouseID, variantID, in.OutputQuantity,
		in.Notes, userID).Scan(&prodID)
	if err != nil {
		return "", fmt.Errorf("create production order: %w", err)
	}

	// Initial status
	_, err = tx.Exec(`
		INSERT INTO production_status_history (production_order_id, status, notes, updated_by)
		VALUES ($1,'PLANNED','Production order created',$2)
	`, prodID, userID)
	if err != nil {
		return "", fmt.Errorf("insert initial status: %w", err)
	}

	// Insert materials
	for _, m := range in.Materials {
		if m.QuantityUsed <= 0 {
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO production_materials (production_order_id, raw_material_stock_id, quantity_used)
			VALUES ($1,$2,$3)
		`, prodID, m.RawMaterialStockID, m.QuantityUsed)
		if err != nil {
			return "", fmt.Errorf("insert production material: %w", err)
		}
	}

	return prodID, tx.Commit()
}

// ──────────────────────────────────────────
// Push status
// ──────────────────────────────────────────

func (s *Store) PushStatus(prodID string, in StatusUpdateInput, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO production_status_history (production_order_id, status, notes, updated_by)
		VALUES ($1,$2,$3,$4)
	`, prodID, in.Status, in.Notes, userID)
	if err != nil {
		return fmt.Errorf("insert status: %w", err)
	}

	startedClause := ""
	if in.Status == "IN_PROGRESS" {
		startedClause = ", started_at = NOW()"
	}

	_, err = tx.Exec(fmt.Sprintf(`
		UPDATE production_orders SET status = $1, updated_at = NOW()%s WHERE id = $2
	`, startedClause), in.Status, prodID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// Complete — triggers stock movements
// ──────────────────────────────────────────

func (s *Store) Complete(prodID, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status, whID, outputVariantID string
	var outputQty float64
	err = tx.QueryRow(`
		SELECT status, warehouse_id, output_variant_id, output_quantity
		FROM production_orders WHERE id = $1
	`, prodID).Scan(&status, &whID, &outputVariantID, &outputQty)
	if err != nil {
		return err
	}
	if status == "COMPLETED" {
		return fmt.Errorf("production order already completed")
	}
	if status == "CANCELLED" {
		return fmt.Errorf("cannot complete a cancelled production order")
	}

	// Deduct raw materials
	matRows, err := tx.Query(`SELECT raw_material_stock_id, quantity_used FROM production_materials WHERE production_order_id = $1`, prodID)
	if err != nil {
		return err
	}
	type matItem struct {
		stockID string
		qty     float64
	}
	var mats []matItem
	for matRows.Next() {
		var m matItem
		if err := matRows.Scan(&m.stockID, &m.qty); err != nil {
			matRows.Close()
			return err
		}
		mats = append(mats, m)
	}
	matRows.Close()

	for _, m := range mats {
		// Look up item_name and warehouse_id from raw_material_stocks
		var itemName, rmWhID string
		err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, m.stockID).Scan(&itemName, &rmWhID)
		if err != nil {
			return fmt.Errorf("raw material stock not found: %w", err)
		}

		res, err := tx.Exec(`
			UPDATE raw_material_stocks SET quantity = quantity - $1, updated_at = NOW()
			WHERE id = $2 AND quantity >= $1
		`, m.qty, m.stockID)
		if err != nil {
			return fmt.Errorf("deduct raw material: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("insufficient raw material stock for %s", itemName)
		}
		_, err = tx.Exec(`
			INSERT INTO raw_material_movements
				(item_name, warehouse_id, quantity, movement_type, reference)
			VALUES ($1,$2,$3,'OUT',$4)
		`, itemName, rmWhID, m.qty, "PROD:"+prodID)
		if err != nil {
			return err
		}
	}

	// Add finished product to stock
	_, err = tx.Exec(`
		INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
		VALUES ($1,$2,$3,'PRODUCT',NOW())
		ON CONFLICT (variant_id, warehouse_id)
		DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
	`, outputVariantID, whID, outputQty)
	if err != nil {
		return fmt.Errorf("add output stock: %w", err)
	}
	_, err = tx.Exec(`
		INSERT INTO stock_movements (variant_id, to_warehouse_id, quantity, movement_type, reference, status)
		VALUES ($1,$2,$3,'PRODUCTION_IN',$4,'COMPLETED')
	`, outputVariantID, whID, outputQty, "PROD:"+prodID)
	if err != nil {
		return err
	}

	// Update order
	_, err = tx.Exec(`
		UPDATE production_orders SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW() WHERE id = $1
	`, prodID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO production_status_history (production_order_id, status, notes, updated_by)
		VALUES ($1,'COMPLETED','Production completed — stock updated',$2)
	`, prodID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// Cancel
// ──────────────────────────────────────────

func (s *Store) Cancel(prodID, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRow(`SELECT status FROM production_orders WHERE id = $1`, prodID).Scan(&status)
	if err != nil {
		return err
	}
	if status == "CANCELLED" {
		return fmt.Errorf("already cancelled")
	}
	if status == "COMPLETED" {
		return fmt.Errorf("cannot cancel a completed production order")
	}

	_, err = tx.Exec(`UPDATE production_orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1`, prodID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO production_status_history (production_order_id, status, notes, updated_by)
		VALUES ($1,'CANCELLED','Cancelled',$2)
	`, prodID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

func (s *Store) List(branchID *string, status, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 0

	if branchID != nil && *branchID != "" {
		n++
		where += fmt.Sprintf(" AND po.branch_id = $%d", n)
		args = append(args, *branchID)
	}
	if status != "" {
		n++
		where += fmt.Sprintf(" AND po.status = $%d", n)
		args = append(args, status)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND (po.production_number ILIKE $%d OR p.name ILIKE $%d)", n, n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*) FROM production_orders po
		LEFT JOIN variants v ON v.id = po.output_variant_id
		LEFT JOIN products p ON p.id = v.product_id
		%s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	limitP := n
	n++
	offsetP := n
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT po.id, po.production_number, po.branch_id, po.warehouse_id,
		       po.output_variant_id, po.output_quantity, po.status,
		       po.notes, po.started_at, po.completed_at, po.created_by, po.created_at,
		       COALESCE(p.name,'') AS product_name, COALESCE(v.sku,'') AS sku,
		       COALESCE(b.name,'') AS branch_name,
		       COALESCE(u.name,'') AS created_by_name
		FROM production_orders po
		LEFT JOIN variants v ON v.id = po.output_variant_id
		LEFT JOIN products p ON p.id = v.product_id
		LEFT JOIN branches b ON b.id = po.branch_id
		LEFT JOIN users u ON u.id = po.created_by
		%s
		ORDER BY po.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitP, offsetP)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var (
			id, prodNum, whID, outputVID, st, notes, createdBy string
			productName, sku, branchName, createdByName        string
			outputQty                                          float64
			branchIDVal                                        sql.NullString
			startedAt, completedAt, createdAt                  sql.NullTime
		)
		if err := rows.Scan(&id, &prodNum, &branchIDVal, &whID,
			&outputVID, &outputQty, &st,
			&notes, &startedAt, &completedAt, &createdBy, &createdAt,
			&productName, &sku, &branchName, &createdByName); err != nil {
			return nil, 0, err
		}
		item := map[string]interface{}{
			"id":                id,
			"production_number": prodNum,
			"branch_id":         branchIDVal.String,
			"branch_name":       branchName,
			"warehouse_id":      whID,
			"output_variant_id": outputVID,
			"output_quantity":   outputQty,
			"product_name":      productName,
			"sku":               sku,
			"status":            st,
			"notes":             notes,
			"started_at":        nil,
			"completed_at":      nil,
			"created_by":        createdBy,
			"created_by_name":   createdByName,
			"created_at":        createdAt.Time,
		}
		if startedAt.Valid {
			item["started_at"] = startedAt.Time
		}
		if completedAt.Valid {
			item["completed_at"] = completedAt.Time
		}
		list = append(list, item)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID — full detail
// ──────────────────────────────────────────

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var (
		prodID, prodNum, whID, outputVID, st, notes, createdBy string
		outputQty                                              float64
		branchIDVal                                            sql.NullString
		startedAt, completedAt, createdAt, updatedAt           sql.NullTime
	)
	err := s.db.QueryRow(`
		SELECT id, production_number, branch_id, warehouse_id,
		       output_variant_id, output_quantity, status,
		       notes, started_at, completed_at, created_by, created_at, updated_at
		FROM production_orders WHERE id = $1
	`, id).Scan(&prodID, &prodNum, &branchIDVal, &whID,
		&outputVID, &outputQty, &st,
		&notes, &startedAt, &completedAt, &createdBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":                prodID,
		"production_number": prodNum,
		"branch_id":         branchIDVal.String,
		"warehouse_id":      whID,
		"output_variant_id": outputVID,
		"output_quantity":   outputQty,
		"status":            st,
		"notes":             notes,
		"started_at":        nil,
		"completed_at":      nil,
		"created_by":        createdBy,
		"created_at":        createdAt.Time,
		"updated_at":        updatedAt.Time,
	}
	if startedAt.Valid {
		result["started_at"] = startedAt.Time
	}
	if completedAt.Valid {
		result["completed_at"] = completedAt.Time
	}

	// Output product info
	var productName, sku string
	if err := s.db.QueryRow(`
		SELECT COALESCE(p.name,''), COALESCE(v.sku,'')
		FROM variants v JOIN products p ON p.id = v.product_id
		WHERE v.id = $1
	`, outputVID).Scan(&productName, &sku); err == nil {
		result["output_product_name"] = productName
		result["output_sku"] = sku
	}

	// Materials
	matRows, err := s.db.Query(`
		SELECT pm.id, pm.raw_material_stock_id, pm.quantity_used,
		       rms.item_name, COALESCE(rms.unit,'') AS unit,
		       COALESCE(w.name,'') AS warehouse_name
		FROM production_materials pm
		LEFT JOIN raw_material_stocks rms ON rms.id = pm.raw_material_stock_id
		LEFT JOIN warehouses w ON w.id = rms.warehouse_id
		WHERE pm.production_order_id = $1
	`, id)
	if err == nil {
		defer matRows.Close()
		var mats []map[string]interface{}
		for matRows.Next() {
			var mid, stockID, itemName, unit, whName string
			var mqty float64
			if err := matRows.Scan(&mid, &stockID, &mqty, &itemName, &unit, &whName); err == nil {
				mats = append(mats, map[string]interface{}{
					"id": mid, "raw_material_stock_id": stockID, "quantity_used": mqty,
					"item_name": itemName, "unit": unit, "warehouse_name": whName,
				})
			}
		}
		if mats == nil {
			mats = []map[string]interface{}{}
		}
		result["materials"] = mats
	}

	// Status history
	shRows, err := s.db.Query(`
		SELECT sh.id, sh.status, sh.notes, sh.updated_by, sh.updated_at,
		       COALESCE(u.name,'') AS updated_by_name
		FROM production_status_history sh
		LEFT JOIN users u ON u.id = sh.updated_by
		WHERE sh.production_order_id = $1
		ORDER BY sh.updated_at ASC
	`, id)
	if err == nil {
		defer shRows.Close()
		var history []map[string]interface{}
		for shRows.Next() {
			var sid, sst, snotes, sby, sname string
			var sat sql.NullTime
			if err := shRows.Scan(&sid, &sst, &snotes, &sby, &sat, &sname); err == nil {
				history = append(history, map[string]interface{}{
					"id": sid, "status": sst, "notes": snotes,
					"updated_by": sby, "updated_by_name": sname, "updated_at": sat.Time,
				})
			}
		}
		if history == nil {
			history = []map[string]interface{}{}
		}
		result["status_history"] = history
	}

	return result, nil
}

// ──────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────

func (s *Store) nextProdNumber(tx *sql.Tx) string {
	var max sql.NullString
	tx.QueryRow(`SELECT MAX(production_number) FROM production_orders WHERE production_number LIKE 'PROD%'`).Scan(&max)
	next := 1
	if max.Valid && len(max.String) > 4 {
		fmt.Sscanf(max.String[4:], "%d", &next)
		next++
	}
	return fmt.Sprintf("PROD%05d", next)
}

func first3(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) >= 3 {
		return s[:3]
	}
	return s
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

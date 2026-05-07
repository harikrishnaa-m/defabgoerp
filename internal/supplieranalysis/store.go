package supplieranalysis

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Get(f Filter) (*SupplierAnalysisResponse, error) {
	if f.Month < 1 || f.Month > 12 {
		return nil, fmt.Errorf("month must be between 1 and 12")
	}
	if f.Year == 0 {
		f.Year = time.Now().Year()
	}

	resp := &SupplierAnalysisResponse{
		SupplierID: f.SupplierID,
		Month:      f.Month,
		Year:       f.Year,
		Items:      []SupplierAnalysisRow{},
	}

	// Mode 1: search by supplier — resolve name and require supplier_id
	if !f.SearchByItem {
		if f.SupplierID == "" {
			return nil, fmt.Errorf("supplier_id is required")
		}
		err := s.db.QueryRow(`SELECT COALESCE(name, '') FROM suppliers WHERE id = $1`, f.SupplierID).Scan(&resp.SupplierName)
		if err != nil {
			return nil, fmt.Errorf("supplier not found")
		}
	} else {
		// Mode 2: search by item — item text is required
		if f.SearchItem == "" {
			return nil, fmt.Errorf("search_item is required when search_by_item is true")
		}
	}

	// Build dynamic WHERE clauses
	args := []interface{}{f.Month, f.Year}
	argIdx := 3

	where := []string{
		`EXTRACT(MONTH FROM po.order_date::date) = $1`,
		`EXTRACT(YEAR  FROM po.order_date::date) = $2`,
	}

	if !f.SearchByItem && f.SupplierID != "" {
		where = append(where, fmt.Sprintf(`po.supplier_id = $%d`, argIdx))
		args = append(args, f.SupplierID)
		argIdx++
	}

	if f.WarehouseID != "" {
		where = append(where, fmt.Sprintf(`po.warehouse_id = $%d`, argIdx))
		args = append(args, f.WarehouseID)
		argIdx++
	}

	if f.SearchItem != "" {
		where = append(where, fmt.Sprintf(`(poi.item_name ILIKE $%d OR poi.product_code ILIKE $%d)`, argIdx, argIdx))
		args = append(args, "%"+f.SearchItem+"%")
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(sup.id::text, ''),
			COALESCE(sup.name, ''),
			poi.item_name,
			poi.quantity,
			COALESCE(poi.unit, ''),
			poi.unit_price,
			COALESCE(po.po_number, ''),
			COALESCE(po.order_date::text, ''),
			COALESCE(pi.discount_amount, 0),
			COALESCE(w.name, ''),
			COALESCE(po.warehouse_id::text, '')
		FROM purchase_order_items poi
		JOIN purchase_orders po        ON po.id  = poi.purchase_order_id
		LEFT JOIN suppliers sup        ON sup.id = po.supplier_id
		LEFT JOIN purchase_invoices pi ON pi.purchase_order_id = po.id
		LEFT JOIN warehouses w         ON w.id   = po.warehouse_id
		WHERE %s
		ORDER BY po.order_date DESC, poi.item_name
	`, strings.Join(where, " AND "))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query supplier analysis: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row SupplierAnalysisRow
		if err := rows.Scan(
			&row.SupplierID,
			&row.SupplierName,
			&row.ItemName,
			&row.Quantity,
			&row.UOM,
			&row.Rate,
			&row.PONumber,
			&row.PODate,
			&row.Discount,
			&row.Location,
			&row.WarehouseID,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		resp.Items = append(resp.Items, row)
	}

	return resp, nil
}

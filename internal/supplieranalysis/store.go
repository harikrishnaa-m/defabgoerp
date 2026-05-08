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
	if f.LastNMonths < 1 {
		f.LastNMonths = 1
	}

	// Date range: start of (current month - (N-1)) to end of current month
	now := time.Now()
	dateFrom := time.Date(now.Year(), now.Month()-time.Month(f.LastNMonths-1), 1, 0, 0, 0, 0, time.UTC)
	dateTo := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Add(-24 * time.Hour)

	resp := &SupplierAnalysisResponse{
		SupplierID:  f.SupplierID,
		LastNMonths: f.LastNMonths,
		DateFrom:    dateFrom.Format("02/01/2006"),
		DateTo:      dateTo.Format("02/01/2006"),
		Items:       []SupplierAnalysisRow{},
	}

	// Mode 1: search by supplier
	if !f.SearchByItem {
		if f.SupplierID == "" {
			return nil, fmt.Errorf("supplier_id is required")
		}
		err := s.db.QueryRow(`SELECT COALESCE(name, '') FROM suppliers WHERE id = $1`, f.SupplierID).Scan(&resp.SupplierName)
		if err != nil {
			return nil, fmt.Errorf("supplier not found")
		}
	} else {
		if f.SearchItem == "" {
			return nil, fmt.Errorf("search_item is required when search_by_item is true")
		}
	}

	// Build dynamic WHERE clauses
	// $1 = dateFrom, $2 = dateTo
	args := []interface{}{dateFrom.Format("2006-01-02"), dateTo.Format("2006-01-02")}
	argIdx := 3

	where := []string{
		`po.order_date::date >= $1::date`,
		`po.order_date::date <= $2::date`,
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
			COALESCE(TO_CHAR(po.order_date, 'DD/MM/YYYY'), ''),
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

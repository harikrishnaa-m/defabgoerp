package hsnreport

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

// ListSales returns all sales invoice line items for a given HSN code and optional date range.
func (s *Store) ListSales(f Filter) (*HSNSalesResult, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	// HSN code is required
	conditions = append(conditions, fmt.Sprintf("LOWER(v.hsn_code) = LOWER($%d)", idx))
	args = append(args, f.HSNCode)
	idx++

	if f.FromDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	base := `
		FROM sales_invoice_items sii
		JOIN variants v         ON v.id  = sii.variant_id
		JOIN sales_invoices si  ON si.id = sii.sales_invoice_id
		LEFT JOIN customers c   ON c.id  = si.customer_id
		LEFT JOIN branches b    ON b.id  = si.branch_id
	`

	// Count + totals
	var total int
	var totalQty, totalAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(sii.quantity), 0), COALESCE(SUM(sii.total_price), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalQty, &totalAmount); err != nil {
		return nil, fmt.Errorf("hsn sales count: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			si.invoice_number,
			TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			COALESCE(c.name, '')        AS customer_name,
			COALESCE(b.name, '')        AS location,
			v.name                      AS variant_name,
			COALESCE(v.hsn_code, '')    AS hsn_code,
			sii.quantity,
			sii.unit_price,
			sii.discount,
			sii.tax_percent,
			sii.tax_amount,
			sii.total_price
		%s %s
		ORDER BY si.invoice_date DESC, si.invoice_number DESC
	`, base, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("hsn sales query: %w", err)
	}
	defer rows.Close()

	var data []HSNSalesRow
	for rows.Next() {
		var row HSNSalesRow
		if err := rows.Scan(
			&row.InvoiceNumber,
			&row.Date,
			&row.CustomerName,
			&row.Location,
			&row.VariantName,
			&row.HSNCode,
			&row.Quantity,
			&row.UnitPrice,
			&row.Discount,
			&row.TaxPercent,
			&row.TaxAmount,
			&row.TotalPrice,
		); err != nil {
			return nil, fmt.Errorf("hsn sales scan: %w", err)
		}
		data = append(data, row)
	}
	if data == nil {
		data = []HSNSalesRow{}
	}

	return &HSNSalesResult{
		Data:          data,
		Total:         total,
		TotalQuantity: totalQty,
		TotalAmount:   totalAmount,
	}, nil
}

// ListJobOrders returns all job order material line items for a given HSN code and optional date range.
func (s *Store) ListJobOrders(f Filter) (*HSNJobOrderResult, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	// HSN code is required — match on the variant
	conditions = append(conditions, fmt.Sprintf("LOWER(v.hsn_code) = LOWER($%d)", idx))
	args = append(args, f.HSNCode)
	idx++

	if f.FromDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(jo.received_date AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(jo.received_date AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	base := `
		FROM job_order_materials jom
		JOIN variants v          ON v.id  = jom.variant_id
		JOIN job_orders jo       ON jo.id = jom.job_order_id
		LEFT JOIN customers c    ON c.id  = jo.customer_id
		LEFT JOIN branches b     ON b.id  = jo.branch_id
	`

	// Count + totals
	var total int
	var totalQty, totalAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(jom.quantity_used), 0), COALESCE(SUM(jo.net_amount), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalQty, &totalAmount); err != nil {
		return nil, fmt.Errorf("hsn joborder count: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			jo.job_number,
			TO_CHAR(jo.received_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			COALESCE(c.name, '')     AS customer_name,
			COALESCE(b.name, '')     AS location,
			jo.job_type,
			jo.status,
			v.name                   AS variant_name,
			COALESCE(v.hsn_code, '') AS hsn_code,
			jom.quantity_used,
			jo.net_amount
		%s %s
		ORDER BY jo.received_date DESC, jo.job_number DESC
	`, base, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("hsn joborder query: %w", err)
	}
	defer rows.Close()

	var data []HSNJobOrderRow
	for rows.Next() {
		var row HSNJobOrderRow
		if err := rows.Scan(
			&row.JobNumber,
			&row.Date,
			&row.CustomerName,
			&row.Location,
			&row.JobType,
			&row.Status,
			&row.VariantName,
			&row.HSNCode,
			&row.QuantityUsed,
			&row.NetAmount,
		); err != nil {
			return nil, fmt.Errorf("hsn joborder scan: %w", err)
		}
		data = append(data, row)
	}
	if data == nil {
		data = []HSNJobOrderRow{}
	}

	return &HSNJobOrderResult{
		Data:          data,
		Total:         total,
		TotalQuantity: totalQty,
		TotalAmount:   totalAmount,
	}, nil
}

// ListPurchase returns all purchase order line items for a given HSN code and optional date range.
func (s *Store) ListPurchase(f Filter) (*HSNPurchaseResult, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	// HSN code is required
	conditions = append(conditions, fmt.Sprintf("LOWER(poi.hsn_code) = LOWER($%d)", idx))
	args = append(args, f.HSNCode)
	idx++

	if f.FromDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"po.order_date::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conditions = append(conditions, fmt.Sprintf(
			"po.order_date::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	base := `
		FROM purchase_order_items poi
		JOIN purchase_orders po  ON po.id  = poi.purchase_order_id
		LEFT JOIN suppliers s    ON s.id   = po.supplier_id
	`

	// Count + totals
	var total int
	var totalQty, totalAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(poi.quantity), 0), COALESCE(SUM(poi.total_price), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalQty, &totalAmount); err != nil {
		return nil, fmt.Errorf("hsn purchase count: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			po.po_number,
			TO_CHAR(po.order_date, 'DD/MM/YYYY')     AS date,
			COALESCE(s.name, '')                     AS supplier_name,
			poi.item_name,
			COALESCE(poi.hsn_code, '')               AS hsn_code,
			COALESCE(poi.unit, '')                   AS unit,
			poi.quantity,
			poi.unit_price,
			COALESCE(poi.gst_percent, 0)             AS gst_percent,
			COALESCE(poi.gst_amount, 0)              AS gst_amount,
			poi.total_price
		%s %s
		ORDER BY po.order_date DESC, po.po_number DESC
	`, base, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("hsn purchase query: %w", err)
	}
	defer rows.Close()

	var data []HSNPurchaseRow
	for rows.Next() {
		var row HSNPurchaseRow
		if err := rows.Scan(
			&row.PONumber,
			&row.Date,
			&row.SupplierName,
			&row.ItemName,
			&row.HSNCode,
			&row.Unit,
			&row.Quantity,
			&row.UnitPrice,
			&row.GSTPercent,
			&row.GSTAmount,
			&row.TotalAmount,
		); err != nil {
			return nil, fmt.Errorf("hsn purchase scan: %w", err)
		}
		data = append(data, row)
	}
	if data == nil {
		data = []HSNPurchaseRow{}
	}

	return &HSNPurchaseResult{
		Data:          data,
		Total:         total,
		TotalQuantity: totalQty,
		TotalAmount:   totalAmount,
	}, nil
}

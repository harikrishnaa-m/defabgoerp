package salesreport

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

func (s *Store) List(f Filter) (*ReportResult, error) {
	limit := f.Limit // 0 means no pagination — return all
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	base := `
		FROM sales_invoices si
		LEFT JOIN customers c     ON c.id  = si.customer_id
		LEFT JOIN branches b      ON b.id  = si.branch_id
		LEFT JOIN sales_orders so ON so.id = si.sales_order_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN users u         ON u.id::text = si.created_by::text
	`

	var conditions []string
	var args []interface{}
	idx := 1

	if f.BranchID != "" {
		conditions = append(conditions, fmt.Sprintf("si.branch_id = $%d", idx))
		args = append(args, f.BranchID)
		idx++
	}
	if f.FromDate != "" {
		conditions = append(conditions, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conditions = append(conditions, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}
	if f.SalespersonID != "" {
		conditions = append(conditions, fmt.Sprintf("so.salesperson_id = $%d", idx))
		args = append(args, f.SalespersonID)
		idx++
	}
	if f.CreatedByID != "" {
		conditions = append(conditions, fmt.Sprintf("si.created_by::text = $%d", idx))
		args = append(args, f.CreatedByID)
		idx++
	}
	if f.PaymentType != "" {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM sales_payments spm WHERE spm.sales_invoice_id = si.id AND spm.payment_method = $%d)", idx))
		args = append(args, f.PaymentType)
		idx++
	}
	if f.Channel != "" {
		conditions = append(conditions, fmt.Sprintf("si.channel = $%d", idx))
		args = append(args, f.Channel)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count + grand total net amount
	var total int
	var totalNetAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(si.net_amount), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalNetAmount); err != nil {
		return nil, err
	}

	totalPages := 1
	if limit > 0 && total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	// Paginated list
	var query string
	if limit > 0 {
		query = fmt.Sprintf(`
		SELECT
			si.id,
			si.invoice_number,
			TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			COALESCE(c.name, '') AS customer_name,
			si.net_amount,
			COALESCE(b.name, '') AS location,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(u.name, '') AS created_by_name,
			COALESCE(si.channel, '') AS channel
		%s %s
		ORDER BY si.invoice_date DESC, si.invoice_number DESC
		LIMIT $%d OFFSET $%d
		`, base, where, idx, idx+1)
		args = append(args, limit, offset)
	} else {
		query = fmt.Sprintf(`
		SELECT
			si.id,
			si.invoice_number,
			TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			COALESCE(c.name, '') AS customer_name,
			si.net_amount,
			COALESCE(b.name, '') AS location,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(u.name, '') AS created_by_name,
			COALESCE(si.channel, '') AS channel
		%s %s
		ORDER BY si.invoice_date DESC, si.invoice_number DESC
		`, base, where)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []SalesReportRow
	for rows.Next() {
		var row SalesReportRow
		if err := rows.Scan(
			&row.ID,
			&row.InvoiceNumber,
			&row.Date,
			&row.CustomerName,
			&row.NetAmount,
			&row.Location,
			&row.SalespersonName,
			&row.CreatedByName,
			&row.Channel,
		); err != nil {
			return nil, err
		}
		data = append(data, row)
	}
	if data == nil {
		data = []SalesReportRow{}
	}

	return &ReportResult{
		Data:           data,
		Total:          total,
		Page:           page,
		Limit:          limit,
		TotalPages:     totalPages,
		TotalNetAmount: totalNetAmount,
	}, nil
}

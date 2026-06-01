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
		FROM sales_payments spm
		JOIN sales_invoices si    ON si.id  = spm.sales_invoice_id AND si.status NOT IN ('RETURNED','CANCELLED')
		LEFT JOIN customers c     ON c.id   = si.customer_id
		LEFT JOIN branches b      ON b.id   = si.branch_id
		LEFT JOIN sales_orders so ON so.id  = si.sales_order_id
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
		if f.PaymentType == "CARD" {
			// CARD covers CARD, DEBIT_CARD, and CREDIT_CARD
			conditions = append(conditions, fmt.Sprintf("spm.payment_method = ANY($%d)", idx))
			args = append(args, []string{"CARD", "DEBIT_CARD", "CREDIT_CARD"})
		} else {
			conditions = append(conditions, fmt.Sprintf("spm.payment_method = $%d", idx))
			args = append(args, f.PaymentType)
		}
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

	// Count + grand total (sum of payment amounts minus any refunds)
	var total int
	var totalNetAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(
			spm.amount - COALESCE((
				SELECT SUM(rp.amount)
				FROM return_orders ro
				JOIN return_payments rp ON rp.return_order_id = ro.id
				WHERE ro.sales_invoice_id = si.id AND ro.status != 'CANCELLED'
			), 0)
		), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalNetAmount); err != nil {
		return nil, err
	}

	totalPages := 1
	if limit > 0 && total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	select_ := `
		SELECT
			spm.id,
			si.invoice_number,
			TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			COALESCE(c.name, '') AS customer_name,
			spm.amount - COALESCE((
				SELECT SUM(rp.amount)
				FROM return_orders ro
				JOIN return_payments rp ON rp.return_order_id = ro.id
				WHERE ro.sales_invoice_id = si.id AND ro.status != 'CANCELLED'
			), 0) AS amount,
			spm.payment_method,
			COALESCE(b.name, '') AS location,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(u.name, '') AS created_by_name,
			COALESCE(si.channel, '') AS channel
	`

	query := fmt.Sprintf(`%s %s %s ORDER BY si.invoice_date DESC, si.invoice_number DESC`, select_, base, where)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, limit, offset)
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
			&row.PaymentMethod,
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

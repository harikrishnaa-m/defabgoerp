package paymentreport

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
	limit := f.Limit // 0 = no pagination, return all
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	base := `
		FROM sales_payments sp
		JOIN sales_invoices si ON si.id = sp.sales_invoice_id
		LEFT JOIN customers c  ON c.id  = si.customer_id
		LEFT JOIN branches b   ON b.id  = si.branch_id
	`

	var conditions []string
	var args []interface{}
	idx := 1

	if f.BranchID != "" {
		conditions = append(conditions, fmt.Sprintf("si.branch_id = $%d", idx))
		args = append(args, f.BranchID)
		idx++
	}
	if f.PaymentMethod != "" {
		if f.PaymentMethod == "CARD" {
			// CARD covers CARD, DEBIT_CARD, and CREDIT_CARD
			conditions = append(conditions, fmt.Sprintf("sp.payment_method = ANY($%d)", idx))
			args = append(args, []string{"CARD", "DEBIT_CARD", "CREDIT_CARD"})
		} else {
			conditions = append(conditions, fmt.Sprintf("sp.payment_method = $%d", idx))
			args = append(args, f.PaymentMethod)
		}
		idx++
	}
	if f.FromDate != "" {
		conditions = append(conditions, fmt.Sprintf("(sp.paid_at AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conditions = append(conditions, fmt.Sprintf("(sp.paid_at AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count + total amount
	var total int
	var totalAmount float64
	if err := s.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(sp.amount), 0) %s %s`, base, where),
		args...,
	).Scan(&total, &totalAmount); err != nil {
		return nil, err
	}

	totalPages := 1
	if total > 0 && limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	// Build select query
	sel := fmt.Sprintf(`
		SELECT
			sp.id,
			sp.payment_method,
			COALESCE(sp.reference, sp.payment_method) AS transaction_no,
			sp.amount,
			TO_CHAR(sp.paid_at AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
			si.invoice_number,
			COALESCE(c.name, '') AS customer_name,
			COALESCE(b.name, '') AS location
		%s %s
		ORDER BY sp.paid_at DESC
	`, base, where)

	if limit > 0 {
		sel += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, limit, offset)
	}

	rows, err := s.db.Query(sel, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []PaymentReportRow
	for rows.Next() {
		var row PaymentReportRow
		if err := rows.Scan(
			&row.ID,
			&row.TxnType,
			&row.TransactionNo,
			&row.Amount,
			&row.Date,
			&row.InvoiceNumber,
			&row.CustomerName,
			&row.Location,
		); err != nil {
			return nil, err
		}
		data = append(data, row)
	}
	if data == nil {
		data = []PaymentReportRow{}
	}

	return &ReportResult{
		Data:        data,
		Total:       total,
		Page:        page,
		Limit:       limit,
		TotalPages:  totalPages,
		TotalAmount: totalAmount,
	}, nil
}

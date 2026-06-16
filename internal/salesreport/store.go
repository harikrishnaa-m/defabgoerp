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

// List dispatches to the correct query mode:
//   - PaymentType set → payment-filtered (net_amount = matched payment amount)
//   - PaymentType empty → invoice-based (net_amount = invoice value, credit notes as negative rows)
func (s *Store) List(f Filter) (*ReportResult, error) {
	if f.PaymentType != "" {
		return s.listByPayment(f)
	}
	return s.listByInvoice(f)
}

// ─────────────────────────────────────────────────────────────────────────────
// listByInvoice — invoice-driven, one row per invoice.
// Exchange credit notes appear as negative rows (channel = CREDIT_NOTE).
// Payment columns (cash/card/upi/bank_transfer/exchange_credit) show the
// amount collected per method on each invoice.
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) listByInvoice(f Filter) (*ReportResult, error) {
	limit := f.Limit
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	var invConds []string
	var cnConds []string
	var args []interface{}
	idx := 1
	// When salesperson or channel filter is active, credit notes are excluded
	// because they carry no salesperson and their channel is always CREDIT_NOTE.
	includeCreditNotes := true

	if f.BranchID != "" {
		invConds = append(invConds, fmt.Sprintf("si.branch_id = $%d", idx))
		cnConds = append(cnConds, fmt.Sprintf("ro.branch_id = $%d", idx))
		args = append(args, f.BranchID)
		idx++
	}
	if f.FromDate != "" {
		invConds = append(invConds, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		cnConds = append(cnConds, fmt.Sprintf("(ro.created_at AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		invConds = append(invConds, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		cnConds = append(cnConds, fmt.Sprintf("(ro.created_at AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}
	if f.SalespersonID != "" {
		invConds = append(invConds, fmt.Sprintf("so.salesperson_id = $%d", idx))
		args = append(args, f.SalespersonID)
		idx++
		includeCreditNotes = false
	}
	if f.CreatedByID != "" {
		invConds = append(invConds, fmt.Sprintf("si.created_by::text = $%d", idx))
		cnConds = append(cnConds, fmt.Sprintf("ro.created_by::text = $%d", idx))
		args = append(args, f.CreatedByID)
		idx++
	}
	if f.Channel != "" {
		invConds = append(invConds, fmt.Sprintf("si.channel = $%d", idx))
		args = append(args, f.Channel)
		idx++
		includeCreditNotes = false
	}

	invWhere := "WHERE si.status NOT IN ('CANCELLED')"
	if len(invConds) > 0 {
		invWhere += " AND " + strings.Join(invConds, " AND ")
	}

	creditNoteUnion := ""
	if includeCreditNotes {
		cnWhere := "WHERE ro.source = 'EXCHANGE' AND ro.status != 'CANCELLED'"
		if len(cnConds) > 0 {
			cnWhere += " AND " + strings.Join(cnConds, " AND ")
		}
		creditNoteUnion = fmt.Sprintf(`
    UNION ALL
    SELECT
        ro.id,
        ro.return_number            AS invoice_number,
        TO_CHAR(ro.created_at AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
        COALESCE(c.name, '')        AS customer_name,
        'CREDIT_NOTE'               AS channel,
        -ro.total_amount            AS net_amount,
        -ro.gst_amount              AS gst_amount,
        NULL::NUMERIC               AS cash,
        NULL::NUMERIC               AS card,
        NULL::NUMERIC               AS upi,
        NULL::NUMERIC               AS bank_transfer,
        NULL::NUMERIC               AS exchange_credit,
        COALESCE(b.name, '')        AS location,
        ''                          AS salesperson_name,
        COALESCE(u.name, '')        AS created_by_name,
        ro.created_at               AS sort_date
    FROM return_orders ro
    LEFT JOIN customers c ON c.id = ro.customer_id
    LEFT JOIN branches  b ON b.id = ro.branch_id
    LEFT JOIN users     u ON u.id::text = ro.created_by::text
    %s`, cnWhere)
	}

	query := fmt.Sprintf(`
WITH combined AS (
    SELECT
        si.id,
        si.invoice_number,
        TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
        COALESCE(c.name,  '')       AS customer_name,
        COALESCE(si.channel, '')    AS channel,
        si.net_amount + COALESCE((
            SELECT SUM(ro.total_amount)
            FROM return_orders ro
            WHERE ro.sales_invoice_id = si.id
              AND ro.source != 'EXCHANGE'
              AND ro.status  != 'CANCELLED'
        ), 0) AS net_amount,
        si.gst_amount + COALESCE((
            SELECT SUM(ro.gst_amount)
            FROM return_orders ro
            WHERE ro.sales_invoice_id = si.id
              AND ro.source != 'EXCHANGE'
              AND ro.status  != 'CANCELLED'
        ), 0) AS gst_amount,
        NULLIF(COALESCE(SUM(CASE WHEN spm.payment_method = 'CASH'                                         THEN spm.amount ELSE 0 END), 0), 0) AS cash,
        NULLIF(COALESCE(SUM(CASE WHEN spm.payment_method IN ('CARD','DEBIT_CARD','CREDIT_CARD')           THEN spm.amount ELSE 0 END), 0), 0) AS card,
        NULLIF(COALESCE(SUM(CASE WHEN spm.payment_method = 'UPI'                                          THEN spm.amount ELSE 0 END), 0), 0) AS upi,
        NULLIF(COALESCE(SUM(CASE WHEN spm.payment_method = 'BANK_TRANSFER'                               THEN spm.amount ELSE 0 END), 0), 0) AS bank_transfer,
        NULLIF(COALESCE(SUM(CASE WHEN spm.payment_method = 'EXCHANGE_CREDIT'                             THEN spm.amount ELSE 0 END), 0), 0) AS exchange_credit,
        COALESCE(b.name,  '')       AS location,
        COALESCE(sp.name, '')       AS salesperson_name,
        COALESCE(u.name,  '')       AS created_by_name,
        si.invoice_date             AS sort_date
    FROM sales_invoices si
    LEFT JOIN customers    c  ON c.id  = si.customer_id
    LEFT JOIN branches     b  ON b.id  = si.branch_id
    LEFT JOIN sales_orders so ON so.id = si.sales_order_id
    LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
    LEFT JOIN users        u  ON u.id::text = si.created_by::text
    LEFT JOIN sales_payments spm ON spm.sales_invoice_id = si.id
    %s
    GROUP BY si.id, si.invoice_number, si.invoice_date, si.net_amount, si.gst_amount,
             c.name, b.name, sp.name, u.name, si.channel
    %s
)
SELECT
    id, invoice_number, date, customer_name, channel,
    net_amount, gst_amount,
    cash, card, upi, bank_transfer, exchange_credit,
    location, salesperson_name, created_by_name,
    COUNT(*)                             OVER ()  AS total_count,
    COALESCE(SUM(net_amount)             OVER (), 0) AS total_net,
    COALESCE(SUM(gst_amount)             OVER (), 0) AS total_gst,
    COALESCE(SUM(cash)                   OVER (), 0) AS total_cash,
    COALESCE(SUM(card)                   OVER (), 0) AS total_card,
    COALESCE(SUM(upi)                    OVER (), 0) AS total_upi,
    COALESCE(SUM(bank_transfer)          OVER (), 0) AS total_bank_transfer,
    COALESCE(SUM(exchange_credit)        OVER (), 0) AS total_exchange_credit
FROM combined
ORDER BY sort_date DESC, invoice_number DESC`, invWhere, creditNoteUnion)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, limit, offset)
	}

	return s.scanRows(query, args, page, limit)
}

// ─────────────────────────────────────────────────────────────────────────────
// listByPayment — payment-filtered view.
// One row per invoice; net_amount = amount collected by the filtered method.
// Only invoices that have at least one payment of that type are shown.
// Credit notes (no cash payments) are excluded.
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) listByPayment(f Filter) (*ReportResult, error) {
	limit := f.Limit
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	var args []interface{}
	idx := 1

	// Build payment-filter CASE expression (reused in SELECT and HAVING)
	var payExpr string
	if f.PaymentType == "CARD" {
		args = append(args, []string{"CARD", "DEBIT_CARD", "CREDIT_CARD"})
		payExpr = fmt.Sprintf("CASE WHEN spm.payment_method = ANY($%d) THEN spm.amount ELSE 0 END", idx)
		idx++
	} else {
		args = append(args, f.PaymentType)
		payExpr = fmt.Sprintf("CASE WHEN spm.payment_method = $%d THEN spm.amount ELSE 0 END", idx)
		idx++
	}

	var conditions []string
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
	if f.Channel != "" {
		conditions = append(conditions, fmt.Sprintf("si.channel = $%d", idx))
		args = append(args, f.Channel)
		idx++
	}

	where := "WHERE si.status NOT IN ('CANCELLED')"
	if len(conditions) > 0 {
		where += " AND " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
WITH base AS (
    SELECT
        si.id,
        si.invoice_number,
        TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
        COALESCE(c.name,  '')    AS customer_name,
        COALESCE(si.channel, '') AS channel,
        SUM(%s)                  AS net_amount,
        si.gst_amount,
        NULLIF(SUM(CASE WHEN spm.payment_method = 'CASH'                                        THEN spm.amount ELSE 0 END), 0) AS cash,
        NULLIF(SUM(CASE WHEN spm.payment_method IN ('CARD','DEBIT_CARD','CREDIT_CARD')          THEN spm.amount ELSE 0 END), 0) AS card,
        NULLIF(SUM(CASE WHEN spm.payment_method = 'UPI'                                         THEN spm.amount ELSE 0 END), 0) AS upi,
        NULLIF(SUM(CASE WHEN spm.payment_method = 'BANK_TRANSFER'                              THEN spm.amount ELSE 0 END), 0) AS bank_transfer,
        NULL::NUMERIC            AS exchange_credit,
        COALESCE(b.name,  '')    AS location,
        COALESCE(sp.name, '')    AS salesperson_name,
        COALESCE(u.name,  '')    AS created_by_name,
        si.invoice_date          AS sort_date
    FROM sales_invoices si
    JOIN  sales_payments   spm ON spm.sales_invoice_id = si.id
                               AND spm.payment_method != 'EXCHANGE_CREDIT'
    LEFT JOIN customers    c  ON c.id  = si.customer_id
    LEFT JOIN branches     b  ON b.id  = si.branch_id
    LEFT JOIN sales_orders so ON so.id = si.sales_order_id
    LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
    LEFT JOIN users        u  ON u.id::text = si.created_by::text
    %s
    GROUP BY si.id, si.invoice_number, si.invoice_date, si.gst_amount,
             c.name, b.name, sp.name, u.name, si.channel
    HAVING SUM(%s) > 0
)
SELECT
    id, invoice_number, date, customer_name, channel,
    net_amount, gst_amount,
    cash, card, upi, bank_transfer, exchange_credit,
    location, salesperson_name, created_by_name,
    COUNT(*)                             OVER ()  AS total_count,
    COALESCE(SUM(net_amount)             OVER (), 0) AS total_net,
    COALESCE(SUM(gst_amount)             OVER (), 0) AS total_gst,
    COALESCE(SUM(cash)                   OVER (), 0) AS total_cash,
    COALESCE(SUM(card)                   OVER (), 0) AS total_card,
    COALESCE(SUM(upi)                    OVER (), 0) AS total_upi,
    COALESCE(SUM(bank_transfer)          OVER (), 0) AS total_bank_transfer,
    0::NUMERIC                                       AS total_exchange_credit
FROM base
ORDER BY sort_date DESC, invoice_number DESC`, payExpr, where, payExpr)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, limit, offset)
	}

	return s.scanRows(query, args, page, limit)
}

// ─────────────────────────────────────────────────────────────────────────────
// scanRows is shared by both query modes — scans the standard column set
// including window-function totals appended to every row.
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) scanRows(query string, args []interface{}, page, limit int) (*ReportResult, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []SalesReportRow
	var totalCount int
	var totalNet, totalGST, totalCash, totalCard, totalUPI, totalBank, totalExchange float64

	for rows.Next() {
		var row SalesReportRow
		var cash, card, upi, bank, exchange sql.NullFloat64
		if err := rows.Scan(
			&row.ID, &row.InvoiceNumber, &row.Date, &row.CustomerName, &row.Channel,
			&row.NetAmount, &row.GSTAmount,
			&cash, &card, &upi, &bank, &exchange,
			&row.Location, &row.SalespersonName, &row.CreatedByName,
			&totalCount, &totalNet, &totalGST,
			&totalCash, &totalCard, &totalUPI, &totalBank, &totalExchange,
		); err != nil {
			return nil, err
		}
		row.CGSTAmount = row.GSTAmount / 2
		row.SGSTAmount = row.GSTAmount / 2
		if cash.Valid {
			row.Cash = &cash.Float64
		}
		if card.Valid {
			row.Card = &card.Float64
		}
		if upi.Valid {
			row.UPI = &upi.Float64
		}
		if bank.Valid {
			row.BankTransfer = &bank.Float64
		}
		if exchange.Valid {
			row.ExchangeCredit = &exchange.Float64
		}
		data = append(data, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if data == nil {
		data = []SalesReportRow{}
	}

	totalPages := 1
	if limit > 0 && totalCount > 0 {
		totalPages = (totalCount + limit - 1) / limit
	}

	ptrOrNil := func(v float64) *float64 {
		if v == 0 {
			return nil
		}
		return &v
	}

	totals := ReportTotals{
		NetAmount:      totalNet,
		GSTAmount:      totalGST,
		CGSTAmount:     totalGST / 2,
		SGSTAmount:     totalGST / 2,
		Cash:           ptrOrNil(totalCash),
		Card:           ptrOrNil(totalCard),
		UPI:            ptrOrNil(totalUPI),
		BankTransfer:   ptrOrNil(totalBank),
		ExchangeCredit: ptrOrNil(totalExchange),
	}

	return &ReportResult{
		Data:           data,
		Total:          totalCount,
		Page:           page,
		Limit:          limit,
		TotalPages:     totalPages,
		TotalNetAmount: totalNet,
		Totals:         totals,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ListDetailed — item-level detailed report.
// One row per invoice line item; invoice-level fields repeat across items.
// Totals.NetAmount / GST / discount etc. are summed once per unique invoice.
// Totals.Quantity / ItemTotal / ItemTotalGST are summed across all line items.
// Supports the same filters as List (branch_id, from_date, to_date,
// salesperson_id, created_by_id, payment_type, channel, page, limit).
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) ListDetailed(f Filter) (*DetailedReportResult, error) {
	limit := f.Limit
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := 0
	if limit > 0 {
		offset = (page - 1) * limit
	}

	var conds []string
	var args []interface{}
	idx := 1

	if f.BranchID != "" {
		conds = append(conds, fmt.Sprintf("si.branch_id = $%d", idx))
		args = append(args, f.BranchID)
		idx++
	}
	if f.FromDate != "" {
		conds = append(conds, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date >= $%d::date", idx))
		args = append(args, f.FromDate)
		idx++
	}
	if f.ToDate != "" {
		conds = append(conds, fmt.Sprintf("(si.invoice_date AT TIME ZONE 'Asia/Kolkata')::date <= $%d::date", idx))
		args = append(args, f.ToDate)
		idx++
	}
	if f.SalespersonID != "" {
		conds = append(conds, fmt.Sprintf("so.salesperson_id = $%d", idx))
		args = append(args, f.SalespersonID)
		idx++
	}
	if f.CreatedByID != "" {
		conds = append(conds, fmt.Sprintf("si.created_by::text = $%d", idx))
		args = append(args, f.CreatedByID)
		idx++
	}
	if f.Channel != "" {
		conds = append(conds, fmt.Sprintf("si.channel = $%d", idx))
		args = append(args, f.Channel)
		idx++
	}
	if f.PaymentType != "" {
		if f.PaymentType == "CARD" {
			conds = append(conds, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM sales_payments spm2
				WHERE spm2.sales_invoice_id = si.id
				  AND spm2.payment_method = ANY($%d)
			)`, idx))
			args = append(args, []string{"CARD", "DEBIT_CARD", "CREDIT_CARD"})
		} else {
			conds = append(conds, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM sales_payments spm2
				WHERE spm2.sales_invoice_id = si.id
				  AND spm2.payment_method = $%d
			)`, idx))
			args = append(args, f.PaymentType)
		}
		idx++
	}

	where := "WHERE si.status NOT IN ('CANCELLED')"
	if len(conds) > 0 {
		where += " AND " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
WITH invoice_payments AS (
    SELECT
        sales_invoice_id,
        STRING_AGG(payment_method, ', ' ORDER BY paid_at, id)                         AS payment_methods,
        STRING_AGG(amount::TEXT,   ', ' ORDER BY paid_at, id)                         AS payment_amounts,
        STRING_AGG(COALESCE(reference, ''), ', ' ORDER BY paid_at, id)                AS payment_references,
        NULLIF(SUM(CASE WHEN payment_method = 'CASH'                             THEN amount ELSE 0 END), 0) AS cash,
        NULLIF(SUM(CASE WHEN payment_method IN ('CARD','DEBIT_CARD')              THEN amount ELSE 0 END), 0) AS debit_card,
        NULLIF(SUM(CASE WHEN payment_method = 'CREDIT_CARD'                      THEN amount ELSE 0 END), 0) AS credit_card,
        NULLIF(SUM(CASE WHEN payment_method = 'UPI'                              THEN amount ELSE 0 END), 0) AS upi,
        NULLIF(SUM(CASE WHEN payment_method = 'BANK_TRANSFER'                    THEN amount ELSE 0 END), 0) AS bank_transfer,
        NULLIF(SUM(CASE WHEN payment_method = 'EXCHANGE_CREDIT'                  THEN amount ELSE 0 END), 0) AS exchange_credit
    FROM sales_payments
    GROUP BY sales_invoice_id
),
line_items AS (
    SELECT
        si.id                                                                AS _invoice_id,
        si.invoice_date                                                      AS _invoice_date,
        sii.id                                                               AS _item_id,
        si.invoice_number,
        TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY')  AS date,
        COALESCE(c.name,  '')                                                AS customer_name,
        COALESCE(b.name,  '')                                                AS branch,
        COALESCE(sp.name, '')                                                AS salesperson,
        si.status,
        COALESCE(ip.payment_methods,    '')                                  AS payment_methods,
        COALESCE(ip.payment_amounts,    '')                                  AS payment_amounts,
        COALESCE(ip.payment_references, '')                                  AS payment_references,
        ip.cash,
        ip.debit_card,
        ip.credit_card,
        ip.upi,
        ip.bank_transfer,
        ip.exchange_credit,
        SUM(sii.total_price) OVER (PARTITION BY si.id)                       AS sub_amount,
        COALESCE(si.discount_amount, 0)                                      AS discount_amount,
        COALESCE(si.bill_discount,   0)                                      AS bill_discount,
        ROUND((
            si.gst_amount + COALESCE((
                SELECT SUM(ro.gst_amount) FROM return_orders ro
                WHERE ro.sales_invoice_id = si.id
                  AND ro.source != 'EXCHANGE'
                  AND ro.status != 'CANCELLED'
            ), 0)
        ) / 2, 2)                                                            AS cgst,
        ROUND((
            si.gst_amount + COALESCE((
                SELECT SUM(ro.gst_amount) FROM return_orders ro
                WHERE ro.sales_invoice_id = si.id
                  AND ro.source != 'EXCHANGE'
                  AND ro.status != 'CANCELLED'
            ), 0)
        ) / 2, 2)                                                            AS sgst,
        si.gst_amount + COALESCE((
            SELECT SUM(ro.gst_amount) FROM return_orders ro
            WHERE ro.sales_invoice_id = si.id
              AND ro.source != 'EXCHANGE'
              AND ro.status != 'CANCELLED'
        ), 0)                                                                AS total_gst,
        COALESCE(si.round_off, 0)                                           AS round_off,
        si.net_amount + COALESCE((
            SELECT SUM(ro.total_amount) FROM return_orders ro
            WHERE ro.sales_invoice_id = si.id
              AND ro.source != 'EXCHANGE'
              AND ro.status != 'CANCELLED'
        ), 0)                                                                AS net_amount,
        v.variant_code,
        v.name                                                               AS item_name,
        v.sku,
        COALESCE(v.hsn_code, '')                                             AS hsn_code,
        sii.quantity,
        sii.unit_price,
        sii.discount                                                         AS item_discount,
        sii.tax_percent                                                      AS item_gst_percent,
        ROUND(sii.tax_amount / 2, 2)                                        AS item_cgst,
        ROUND(sii.tax_amount / 2, 2)                                        AS item_sgst,
        sii.tax_amount                                                       AS item_total_gst,
        sii.total_price                                                      AS item_total
    FROM sales_invoices si
    JOIN  sales_invoice_items sii ON sii.sales_invoice_id = si.id
    JOIN  variants            v   ON v.id  = sii.variant_id
    LEFT JOIN invoice_payments ip ON ip.sales_invoice_id = si.id
    LEFT JOIN customers        c  ON c.id  = si.customer_id
    LEFT JOIN branches         b  ON b.id  = si.branch_id
    LEFT JOIN sales_orders     so ON so.id = si.sales_order_id
    LEFT JOIN sales_persons    sp ON sp.id = so.salesperson_id
    %s
),
inv_sums AS (
    SELECT
        SUM(net_amount)      AS s_net,
        SUM(cgst)            AS s_cgst,
        SUM(sgst)            AS s_sgst,
        SUM(total_gst)       AS s_total_gst,
        SUM(discount_amount) AS s_discount,
        SUM(bill_discount)   AS s_bill_discount,
        SUM(round_off)       AS s_round_off,
        NULLIF(SUM(cash),           0) AS s_cash,
        NULLIF(SUM(debit_card),      0) AS s_debit_card,
        NULLIF(SUM(credit_card),     0) AS s_credit_card,
        NULLIF(SUM(upi),             0) AS s_upi,
        NULLIF(SUM(bank_transfer),   0) AS s_bank_transfer,
        NULLIF(SUM(exchange_credit), 0) AS s_exchange_credit
    FROM (
        SELECT DISTINCT _invoice_id, net_amount, cgst, sgst, total_gst, discount_amount, bill_discount, round_off,
                        cash, debit_card, credit_card, upi, bank_transfer, exchange_credit
        FROM line_items
    ) u
)
SELECT
    invoice_number, date, customer_name, branch, salesperson, status,
    payment_methods, payment_amounts, payment_references,
    cash, debit_card, credit_card, upi, bank_transfer, exchange_credit,
    sub_amount, discount_amount, bill_discount,
    cgst, sgst, total_gst, round_off, net_amount,
    variant_code, item_name, sku, hsn_code,
    quantity, unit_price, item_discount, item_gst_percent,
    item_cgst, item_sgst, item_total_gst, item_total,
    COUNT(*)            OVER ()  AS total_count,
    SUM(quantity)       OVER ()  AS total_quantity,
    SUM(item_total)     OVER ()  AS total_item_total,
    SUM(item_total_gst) OVER ()  AS total_item_gst,
    s_net, s_cgst, s_sgst, s_total_gst, s_discount, s_bill_discount, s_round_off,
    s_cash, s_debit_card, s_credit_card, s_upi, s_bank_transfer, s_exchange_credit
FROM line_items CROSS JOIN inv_sums
ORDER BY _invoice_date, invoice_number, _item_id`, where)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, limit, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []DetailedReportRow
	var totalCount int
	var totalQuantity, totalItemTotal, totalItemGST float64
	var sNet, sCGST, sSGST, sTotalGST, sDiscount, sBillDiscount, sRoundOff float64
	var sCash, sDebitCard, sCreditCard, sUPI, sBankTransfer, sExchangeCredit sql.NullFloat64

	for rows.Next() {
		var row DetailedReportRow
		var cash, debitCard, creditCard, upi, bankTransfer, exchangeCredit sql.NullFloat64
		if err := rows.Scan(
			&row.InvoiceNumber,
			&row.Date,
			&row.CustomerName,
			&row.Branch,
			&row.Salesperson,
			&row.Status,
			&row.PaymentMethods,
			&row.PaymentAmounts,
			&row.PaymentRefs,
			&cash,
			&debitCard,
			&creditCard,
			&upi,
			&bankTransfer,
			&exchangeCredit,
			&row.SubAmount,
			&row.DiscountAmount,
			&row.BillDiscount,
			&row.CGST,
			&row.SGST,
			&row.TotalGST,
			&row.RoundOff,
			&row.NetAmount,
			&row.VariantCode,
			&row.ItemName,
			&row.SKU,
			&row.HSNCode,
			&row.Quantity,
			&row.UnitPrice,
			&row.ItemDiscount,
			&row.ItemGSTPercent,
			&row.ItemCGST,
			&row.ItemSGST,
			&row.ItemTotalGST,
			&row.ItemTotal,
			&totalCount,
			&totalQuantity,
			&totalItemTotal,
			&totalItemGST,
			&sNet,
			&sCGST,
			&sSGST,
			&sTotalGST,
			&sDiscount,
			&sBillDiscount,
			&sRoundOff,
			&sCash,
			&sDebitCard,
			&sCreditCard,
			&sUPI,
			&sBankTransfer,
			&sExchangeCredit,
		); err != nil {
			return nil, err
		}
		if cash.Valid {
			row.Cash = &cash.Float64
		}
		if debitCard.Valid {
			row.DebitCard = &debitCard.Float64
		}
		if creditCard.Valid {
			row.CreditCard = &creditCard.Float64
		}
		if upi.Valid {
			row.UPI = &upi.Float64
		}
		if bankTransfer.Valid {
			row.BankTransfer = &bankTransfer.Float64
		}
		if exchangeCredit.Valid {
			row.ExchangeCredit = &exchangeCredit.Float64
		}
		data = append(data, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if data == nil {
		data = []DetailedReportRow{}
	}

	totalPages := 1
	if limit > 0 && totalCount > 0 {
		totalPages = (totalCount + limit - 1) / limit
	}

	ptrOrNil := func(n sql.NullFloat64) *float64 {
		if !n.Valid {
			return nil
		}
		return &n.Float64
	}

	return &DetailedReportResult{
		Data:       data,
		Total:      totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		Totals: DetailedTotals{
			NetAmount:      sNet - sExchangeCredit.Float64,
			DiscountAmount: sDiscount,
			BillDiscount:   sBillDiscount,
			CGST:           sCGST,
			SGST:           sSGST,
			TotalGST:       sTotalGST,
			RoundOff:       sRoundOff,
			Cash:           ptrOrNil(sCash),
			DebitCard:      ptrOrNil(sDebitCard),
			CreditCard:     ptrOrNil(sCreditCard),
			UPI:            ptrOrNil(sUPI),
			BankTransfer:   ptrOrNil(sBankTransfer),
			ExchangeCredit: ptrOrNil(sExchangeCredit),
			Quantity:       totalQuantity,
			ItemTotal:      totalItemTotal,
			ItemTotalGST:   totalItemGST,
		},
	}, nil
}

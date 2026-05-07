package purchasereport

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
	limit := f.Limit
	if limit <= 0 {
		limit = 10
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	// ── base FROM + SELECT ─────────────────────────────────────────────────
	base := `
		FROM purchase_invoices pi
		LEFT JOIN suppliers sup ON sup.id = pi.supplier_id
		LEFT JOIN warehouses w  ON w.id  = pi.warehouse_id
	`

	var conditions []string
	var args []interface{}
	idx := 1

	// top-level filters
	if f.WarehouseID != "" {
		conditions = append(conditions, fmt.Sprintf("pi.warehouse_id = $%d", idx))
		args = append(args, f.WarehouseID)
		idx++
	}
	if f.SupplierID != "" {
		conditions = append(conditions, fmt.Sprintf("pi.supplier_id = $%d", idx))
		args = append(args, f.SupplierID)
		idx++
	}

	// column-level searches
	if f.SearchInvoice != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(pi.invoice_number) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.SearchInvoice)+"%")
		idx++
	}
	if f.SearchDate != "" {
		conditions = append(conditions, fmt.Sprintf("TO_CHAR(pi.invoice_date, 'DD/MM/YYYY') ILIKE $%d", idx))
		args = append(args, "%"+f.SearchDate+"%")
		idx++
	}
	if f.SearchParty != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(sup.name,'')) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.SearchParty)+"%")
		idx++
	}
	if f.SearchPurchaseCost != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(pi.sub_amount,0)::text LIKE $%d", idx))
		args = append(args, "%"+f.SearchPurchaseCost+"%")
		idx++
	}
	if f.SearchRoundOff != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(pi.round_off,0)::text LIKE $%d", idx))
		args = append(args, "%"+f.SearchRoundOff+"%")
		idx++
	}
	if f.SearchOtherCharges != "" {
		conditions = append(conditions, fmt.Sprintf(
			`COALESCE((SELECT SUM(pc.amount) FROM purchase_charges pc WHERE pc.purchase_order_id = pi.purchase_order_id), 0)::text LIKE $%d`, idx))
		args = append(args, "%"+f.SearchOtherCharges+"%")
		idx++
	}
	if f.SearchTax != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(pi.gst_amount,0)::text LIKE $%d", idx))
		args = append(args, "%"+f.SearchTax+"%")
		idx++
	}
	if f.SearchDiscount != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(pi.discount_amount,0)::text LIKE $%d", idx))
		args = append(args, "%"+f.SearchDiscount+"%")
		idx++
	}
	if f.SearchNetAmount != "" {
		conditions = append(conditions, fmt.Sprintf("COALESCE(pi.net_amount,0)::text LIKE $%d", idx))
		args = append(args, "%"+f.SearchNetAmount+"%")
		idx++
	}
	if f.SearchLocation != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(w.name,'')) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.SearchLocation)+"%")
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// ── total count ────────────────────────────────────────────────────────
	var total int
	countQ := "SELECT COUNT(*) " + base + where
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count purchase report: %w", err)
	}

	// ── aggregate totals for all filtered rows ─────────────────────────────
	aggQ := `
		SELECT
			COALESCE(SUM(pi.sub_amount), 0),
			COALESCE(SUM(pi.round_off), 0),
			COALESCE(SUM(
				(SELECT SUM(pc.amount) FROM purchase_charges pc WHERE pc.purchase_order_id = pi.purchase_order_id)
			), 0),
			COALESCE(SUM(pi.gst_amount), 0),
			COALESCE(SUM(pi.discount_amount), 0),
			COALESCE(SUM(pi.net_amount), 0)
	` + base + where

	var t Totals
	if err := s.db.QueryRow(aggQ, args...).Scan(
		&t.PurchaseCostAll, &t.RoundOffAll, &t.OtherChargesAll,
		&t.TaxAll, &t.DiscountAll, &t.NetAmountAll,
	); err != nil {
		return nil, fmt.Errorf("aggregate purchase report: %w", err)
	}

	// ── paginated rows ─────────────────────────────────────────────────────
	selectCols := `
		SELECT
			pi.id::text,
			COALESCE(pi.invoice_number, ''),
			TO_CHAR(pi.invoice_date, 'DD/MM/YYYY'),
			COALESCE(pi.supplier_id::text, ''),
			COALESCE(sup.name, ''),
			COALESCE(pi.sub_amount, 0),
			COALESCE(pi.round_off, 0),
			COALESCE((SELECT SUM(pc.amount) FROM purchase_charges pc WHERE pc.purchase_order_id = pi.purchase_order_id), 0),
			COALESCE(pi.gst_amount, 0),
			COALESCE(pi.discount_amount, 0),
			COALESCE(pi.net_amount, 0),
			COALESCE(pi.warehouse_id::text, ''),
			COALESCE(w.name, '')
	`

	dataQ := selectCols + base + where +
		fmt.Sprintf(" ORDER BY pi.invoice_date DESC, pi.created_at DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.db.Query(dataQ, args...)
	if err != nil {
		return nil, fmt.Errorf("query purchase report: %w", err)
	}
	defer rows.Close()

	data := []PurchaseReportRow{}
	for rows.Next() {
		var r PurchaseReportRow
		if err := rows.Scan(
			&r.ID, &r.InvoiceNumber, &r.BillDate,
			&r.SupplierID, &r.PartyName,
			&r.PurchaseCost, &r.RoundOff, &r.OtherCharges,
			&r.Tax, &r.Discount, &r.NetAmount,
			&r.WarehouseID, &r.Location,
		); err != nil {
			return nil, fmt.Errorf("scan purchase report row: %w", err)
		}
		data = append(data, r)
		// accumulate page totals
		t.PurchaseCostPage += r.PurchaseCost
		t.RoundOffPage += r.RoundOff
		t.OtherChargesPage += r.OtherCharges
		t.TaxPage += r.Tax
		t.DiscountPage += r.Discount
		t.NetAmountPage += r.NetAmount
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate purchase report: %w", err)
	}

	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}

	return &ReportResult{
		Data:       data,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		Totals:     t,
	}, nil
}

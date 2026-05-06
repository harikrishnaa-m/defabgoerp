package supplieraging

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns supplier aging data for the given date range.
// dateFrom / dateTo are YYYY-MM-DD strings.
// Age buckets are computed relative to dateTo.
func (s *Store) Get(dateFrom, dateTo string) (*SupplierAgingResponse, error) {
	if dateTo == "" {
		dateTo = time.Now().Format("2006-01-02")
	}
	if dateFrom == "" {
		dateFrom = time.Now().AddDate(0, -3, 0).Format("2006-01-02")
	}

	// Main query — one row per invoice (to get the PO ref number and location).
	// Age = dateTo - invoice_date (in days).
	// Pending = net_amount - paid_amount - returned_amount (clamped to 0).
	// purchase_returns reduce what is owed to the supplier.
	query := `
		SELECT
			sup.id::text,
			sup.name,
			COALESCE(po.po_number, ''),
			COALESCE(b.name, ''),
			pi.net_amount,
			GREATEST(
				pi.net_amount
				- COALESCE(pi.paid_amount, 0)
				- COALESCE((
					SELECT SUM(pr.net_amount)
					FROM purchase_returns pr
					WHERE pr.purchase_invoice_id = pi.id
				), 0),
				0
			) AS pending,
			(DATE($1) - pi.invoice_date::date) AS age_days
		FROM purchase_invoices pi
		JOIN suppliers sup      ON sup.id = pi.supplier_id
		JOIN purchase_orders po ON po.id  = pi.purchase_order_id
		LEFT JOIN warehouses w  ON w.id   = pi.warehouse_id
		LEFT JOIN branches   b  ON b.id   = w.branch_id
		WHERE pi.invoice_date::date BETWEEN DATE($2) AND DATE($1)
		ORDER BY sup.name, pi.invoice_date
	`

	rows, err := s.db.Query(query, dateTo, dateFrom)
	if err != nil {
		return nil, fmt.Errorf("supplier aging query: %w", err)
	}
	defer rows.Close()

	// aggregate per supplier
	type supplierKey struct {
		id       string
		name     string
		refNo    string
		location string
	}
	type agg struct {
		key     supplierKey
		opening float64
		pending float64
		under7  float64
		d7to15  float64
		d15to31 float64
		over31  float64
	}

	// preserve insertion order
	order := []string{}
	aggMap := map[string]*agg{}

	for rows.Next() {
		var supplierID, supplierName, poNumber, location string
		var netAmount, pending float64
		var ageDays int

		if err := rows.Scan(&supplierID, &supplierName, &poNumber, &location, &netAmount, &pending, &ageDays); err != nil {
			return nil, fmt.Errorf("scan aging row: %w", err)
		}

		a, exists := aggMap[supplierID]
		if !exists {
			a = &agg{key: supplierKey{
				id:       supplierID,
				name:     supplierName,
				refNo:    poNumber,
				location: location,
			}}
			aggMap[supplierID] = a
			order = append(order, supplierID)
		}

		a.opening += netAmount
		a.pending += pending

		switch {
		case ageDays < 7:
			a.under7 += pending
		case ageDays <= 15:
			a.d7to15 += pending
		case ageDays <= 31:
			a.d15to31 += pending
		default:
			a.over31 += pending
		}
	}

	// Build totals (also query total outstanding across all time for footer OF values)
	var totalOpeningAllTime, totalPendingAllTime float64
	_ = s.db.QueryRow(`
		SELECT
			COALESCE(SUM(net_amount), 0),
			COALESCE(SUM(
				GREATEST(
					net_amount
					- COALESCE(paid_amount, 0)
					- COALESCE((SELECT SUM(pr.net_amount) FROM purchase_returns pr WHERE pr.purchase_invoice_id = pi.id), 0),
					0
				)
			), 0)
		FROM purchase_invoices pi
	`).Scan(&totalOpeningAllTime, &totalPendingAllTime)

	var totalUnder7AllTime, totalD7to15AllTime, totalD15to31AllTime, totalOver31AllTime float64
	_ = s.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN (DATE($1) - invoice_date::date) < 7 THEN
				GREATEST(net_amount - COALESCE(paid_amount,0) - COALESCE((SELECT SUM(pr.net_amount) FROM purchase_returns pr WHERE pr.purchase_invoice_id = pi.id),0), 0)
			ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN (DATE($1) - invoice_date::date) BETWEEN 7 AND 15 THEN
				GREATEST(net_amount - COALESCE(paid_amount,0) - COALESCE((SELECT SUM(pr.net_amount) FROM purchase_returns pr WHERE pr.purchase_invoice_id = pi.id),0), 0)
			ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN (DATE($1) - invoice_date::date) BETWEEN 16 AND 31 THEN
				GREATEST(net_amount - COALESCE(paid_amount,0) - COALESCE((SELECT SUM(pr.net_amount) FROM purchase_returns pr WHERE pr.purchase_invoice_id = pi.id),0), 0)
			ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN (DATE($1) - invoice_date::date) > 31 THEN
				GREATEST(net_amount - COALESCE(paid_amount,0) - COALESCE((SELECT SUM(pr.net_amount) FROM purchase_returns pr WHERE pr.purchase_invoice_id = pi.id),0), 0)
			ELSE 0 END), 0)
		FROM purchase_invoices pi
	`, dateTo).Scan(&totalUnder7AllTime, &totalD7to15AllTime, &totalD15to31AllTime, &totalOver31AllTime)

	// Build response rows and filtered totals
	result := &SupplierAgingResponse{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Rows:     []SupplierAgingRow{},
	}

	var totOpening, totPending, totUnder7, totD7to15, totD15to31, totOver31 float64

	for i, id := range order {
		a := aggMap[id]
		result.Rows = append(result.Rows, SupplierAgingRow{
			SlNo:          i + 1,
			SupplierID:    a.key.id,
			SupplierName:  a.key.name,
			SupplierRefNo: a.key.refNo,
			Location:      a.key.location,
			Opening:       a.opening,
			Pending:       a.pending,
			Under7Days:    a.under7,
			Days7To15:     a.d7to15,
			Days15To31:    a.d15to31,
			Over31Days:    a.over31,
		})
		totOpening += a.opening
		totPending += a.pending
		totUnder7 += a.under7
		totD7to15 += a.d7to15
		totD15to31 += a.d15to31
		totOver31 += a.over31
	}

	result.Totals = SupplierAgingTotals{
		Opening:         totOpening,
		TotalOpening:    totalOpeningAllTime,
		Pending:         totPending,
		TotalPending:    totalPendingAllTime,
		Under7Days:      totUnder7,
		TotalUnder7:     totalUnder7AllTime,
		Days7To15:       totD7to15,
		TotalDays7To15:  totalD7to15AllTime,
		Days15To31:      totD15to31,
		Days15To31Total: totalD15to31AllTime,
		Over31Days:      totOver31,
		Over31DaysTotal: totalOver31AllTime,
	}

	return result, nil
}

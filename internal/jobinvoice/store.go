package jobinvoice

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

func (s *Store) List(branchID *string, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 0

	if branchID != nil && *branchID != "" {
		n++
		where += fmt.Sprintf(" AND ji.branch_id = $%d", n)
		args = append(args, *branchID)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND (ji.invoice_number ILIKE $%d OR jo.job_number ILIKE $%d OR c.name ILIKE $%d OR c.phone ILIKE $%d)", n, n, n, n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*) FROM job_invoices ji
		LEFT JOIN job_orders jo ON jo.id = ji.job_order_id
		LEFT JOIN customers c ON c.id = ji.customer_id
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
		SELECT ji.id, ji.invoice_number, ji.job_order_id, jo.job_number,
		       ji.branch_id, COALESCE(b.name,'') AS branch_name,
		       ji.customer_id, COALESCE(c.name,'') AS customer_name, COALESCE(c.phone,'') AS customer_phone,
		       ji.sub_amount, ji.discount_amount, ji.gst_amount, ji.net_amount,
		       ji.payment_status, ji.created_at
		FROM job_invoices ji
		LEFT JOIN job_orders jo ON jo.id = ji.job_order_id
		LEFT JOIN branches b ON b.id = ji.branch_id
		LEFT JOIN customers c ON c.id = ji.customer_id
		%s
		ORDER BY ji.created_at DESC
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
			id, invNum, joID, joNum         string
			custID, custName, custPhone     string
			brID, brName                    sql.NullString
			subAmt, discAmt, gstAmt, netAmt float64
			paySt                           string
			createdAt                       sql.NullTime
		)
		if err := rows.Scan(&id, &invNum, &joID, &joNum,
			&brID, &brName,
			&custID, &custName, &custPhone,
			&subAmt, &discAmt, &gstAmt, &netAmt,
			&paySt, &createdAt); err != nil {
			return nil, 0, err
		}
		list = append(list, map[string]interface{}{
			"id":              id,
			"invoice_number":  invNum,
			"job_order_id":    joID,
			"job_number":      joNum,
			"branch_id":       brID.String,
			"branch_name":     brName.String,
			"customer_id":     custID,
			"customer_name":   custName,
			"customer_phone":  custPhone,
			"sub_amount":      subAmt,
			"discount_amount": discAmt,
			"gst_amount":      gstAmt,
			"net_amount":      netAmt,
			"payment_status":  paySt,
			"created_at":      createdAt.Time,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID
// ──────────────────────────────────────────

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var (
		invID, invNum, joID, joNum             string
		custID, custName, custPhone, custEmail string
		brID, brName                           sql.NullString
		subAmt, discAmt, gstAmt, netAmt        float64
		paySt                                  string
		createdAt                              sql.NullTime
	)
	err := s.db.QueryRow(`
		SELECT ji.id, ji.invoice_number, ji.job_order_id, jo.job_number,
		       ji.branch_id, COALESCE(b.name,'') AS branch_name,
		       ji.customer_id, COALESCE(c.name,'') AS customer_name,
		       COALESCE(c.phone,'') AS customer_phone, COALESCE(c.email,'') AS customer_email,
		       ji.sub_amount, ji.discount_amount, ji.gst_amount, ji.net_amount,
		       ji.payment_status, ji.created_at
		FROM job_invoices ji
		LEFT JOIN job_orders jo ON jo.id = ji.job_order_id
		LEFT JOIN branches b ON b.id = ji.branch_id
		LEFT JOIN customers c ON c.id = ji.customer_id
		WHERE ji.id = $1
	`, id).Scan(&invID, &invNum, &joID, &joNum,
		&brID, &brName,
		&custID, &custName, &custPhone, &custEmail,
		&subAmt, &discAmt, &gstAmt, &netAmt,
		&paySt, &createdAt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":             invID,
		"invoice_number": invNum,
		"job_order_id":   joID,
		"job_number":     joNum,
		"branch_id":      brID.String,
		"branch_name":    brName.String,
		"customer": map[string]interface{}{
			"id":    custID,
			"name":  custName,
			"phone": custPhone,
			"email": custEmail,
		},
		"sub_amount":      subAmt,
		"discount_amount": discAmt,
		"gst_amount":      gstAmt,
		"net_amount":      netAmt,
		"payment_status":  paySt,
		"created_at":      createdAt.Time,
	}

	// Job order items (the invoice line items come from the job order)
	itemRows, err := s.db.Query(`
		SELECT id, description, quantity, unit_price, discount, tax_percent, cgst, sgst, total_price
		FROM job_order_items WHERE job_order_id = $1
	`, joID)
	if err == nil {
		defer itemRows.Close()
		var items []map[string]interface{}
		for itemRows.Next() {
			var iid, desc string
			var qty, up, disc, tp, cgst, sgst, tot float64
			if err := itemRows.Scan(&iid, &desc, &qty, &up, &disc, &tp, &cgst, &sgst, &tot); err == nil {
				items = append(items, map[string]interface{}{
					"id": iid, "description": desc, "quantity": qty,
					"unit_price": up, "discount": disc, "tax_percent": tp,
					"cgst": cgst, "sgst": sgst, "total_price": tot,
				})
			}
		}
		if items == nil {
			items = []map[string]interface{}{}
		}
		result["items"] = items
	}

	// Payments from the job order
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, reference, paid_at
		FROM job_order_payments WHERE job_order_id = $1 ORDER BY paid_at ASC
	`, joID)
	if err == nil {
		defer payRows.Close()
		var payments []map[string]interface{}
		var totalPaid float64
		for payRows.Next() {
			var pid, pm, ref string
			var pamt float64
			var pat sql.NullTime
			if err := payRows.Scan(&pid, &pamt, &pm, &ref, &pat); err == nil {
				payments = append(payments, map[string]interface{}{
					"id": pid, "amount": pamt, "payment_method": pm,
					"reference": ref, "paid_at": pat.Time,
				})
				totalPaid += pamt
			}
		}
		if payments == nil {
			payments = []map[string]interface{}{}
		}
		result["payments"] = payments
		result["total_paid"] = totalPaid
		result["balance_due"] = netAmt - totalPaid
	}

	return result, nil
}

// ──────────────────────────────────────────
// Backfill — create invoices for job orders that don't have one
// ──────────────────────────────────────────

func (s *Store) Backfill() (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT jo.id, jo.branch_id, jo.customer_id,
		       jo.sub_amount, jo.discount_amount, jo.gst_amount, jo.net_amount,
		       jo.payment_status
		FROM job_orders jo
		WHERE NOT EXISTS (
			SELECT 1 FROM job_invoices ji WHERE ji.job_order_id = jo.id
		)
		ORDER BY jo.created_at ASC
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pending struct {
		jobID, custID, paySt            string
		branchID                        sql.NullString
		subAmt, discAmt, gstAmt, netAmt float64
	}
	var items []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.jobID, &p.branchID, &p.custID,
			&p.subAmt, &p.discAmt, &p.gstAmt, &p.netAmt, &p.paySt); err != nil {
			return 0, err
		}
		items = append(items, p)
	}

	for _, p := range items {
		invNum := nextInvoiceNumber(tx)
		var brParam interface{}
		if p.branchID.Valid {
			brParam = p.branchID.String
		}
		_, err = tx.Exec(`
			INSERT INTO job_invoices
				(invoice_number, job_order_id, branch_id, customer_id,
				 sub_amount, discount_amount, gst_amount, net_amount, payment_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, invNum, p.jobID, brParam, p.custID,
			p.subAmt, p.discAmt, p.gstAmt, p.netAmt, p.paySt)
		if err != nil {
			return 0, fmt.Errorf("backfill invoice for job %s: %w", p.jobID, err)
		}
	}

	return len(items), tx.Commit()
}

func nextInvoiceNumber(tx *sql.Tx) string {
	var max sql.NullString
	tx.QueryRow(`SELECT MAX(invoice_number) FROM job_invoices WHERE invoice_number LIKE 'JINV%'`).Scan(&max)
	next := 1
	if max.Valid && len(max.String) > 4 {
		fmt.Sscanf(max.String[4:], "%d", &next)
		next++
	}
	return fmt.Sprintf("JINV%05d", next)
}

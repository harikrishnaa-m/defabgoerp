package purchasereturn

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create creates a purchase return, deducts stock, and records movements.
func (s *Store) Create(in CreatePurchaseReturnInput, userID string) (string, error) {
	if len(in.Items) == 0 {
		return "", errors.New("at least one item is required")
	}
	if in.SupplierID == "" {
		return "", errors.New("supplier_id is required")
	}

	prDate := in.PRDate
	if prDate == "" {
		prDate = time.Now().Format("2006-01-02")
	}
	currency := in.Currency
	if currency == "" {
		currency = "Rs"
	}
	exchangeRate := in.ExchangeRate
	if exchangeRate == 0 {
		exchangeRate = 1
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Generate PR number
	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM purchase_returns`).Scan(&count); err != nil {
		return "", fmt.Errorf("count purchase_returns: %w", err)
	}
	prNumber := fmt.Sprintf("PR-%03d", count+1)

	// Resolve purchase_invoice_id from invoice_number
	var purchaseInvoiceID string
	if in.InvoiceNumber != "" {
		_ = tx.QueryRow(`SELECT id FROM purchase_invoices WHERE invoice_number = $1 LIMIT 1`, in.InvoiceNumber).Scan(&purchaseInvoiceID)
		if purchaseInvoiceID == "" {
			return "", fmt.Errorf("purchase invoice %q not found", in.InvoiceNumber)
		}
	}

	// Auto-resolve GRN from the purchase invoice
	var grnID string
	if purchaseInvoiceID != "" {
		var grnNullable sql.NullString
		_ = tx.QueryRow(
			`SELECT gr.id FROM goods_receipts gr
			 JOIN purchase_orders po ON po.id = gr.purchase_order_id
			 JOIN purchase_invoices pi ON pi.purchase_order_id = po.id
			 WHERE pi.id = $1 LIMIT 1`,
			purchaseInvoiceID,
		).Scan(&grnNullable)
		if grnNullable.Valid {
			grnID = grnNullable.String
		}
	}

	// Also resolve warehouse_id for stock deduction
	var warehouseID string
	if grnID != "" {
		_ = tx.QueryRow(`SELECT warehouse_id FROM goods_receipts WHERE id = $1`, grnID).Scan(&warehouseID)
	}
	if warehouseID == "" {
		// fallback: get from supplier's most recent GRN
		_ = tx.QueryRow(
			`SELECT warehouse_id FROM goods_receipts WHERE supplier_id = $1 ORDER BY received_date DESC LIMIT 1`,
			in.SupplierID,
		).Scan(&warehouseID)
	}

	// Validate return quantities against invoice — prevent over-returning
	if purchaseInvoiceID != "" {
		// invoiced qty per purchase_order_item_id
		type itemQty struct {
			invoiced float64
			returned float64
		}
		qtyMap := map[string]*itemQty{}

		invoiceRows, err := tx.Query(`
			SELECT purchase_order_item_id::text, quantity
			FROM purchase_invoice_items
			WHERE purchase_invoice_id = $1
		`, purchaseInvoiceID)
		if err != nil {
			return "", fmt.Errorf("fetch invoice items for validation: %w", err)
		}
		for invoiceRows.Next() {
			var poItemID string
			var qty float64
			if err := invoiceRows.Scan(&poItemID, &qty); err != nil {
				invoiceRows.Close()
				return "", err
			}
			qtyMap[poItemID] = &itemQty{invoiced: qty}
		}
		invoiceRows.Close()

		// already returned qty per purchase_order_item_id for this invoice
		returnedRows, err := tx.Query(`
			SELECT pri.purchase_order_item_id::text, COALESCE(SUM(pri.quantity), 0)
			FROM purchase_return_items pri
			JOIN purchase_returns pr ON pr.id = pri.purchase_return_id
			WHERE pr.purchase_invoice_id = $1
			GROUP BY pri.purchase_order_item_id
		`, purchaseInvoiceID)
		if err != nil {
			return "", fmt.Errorf("fetch existing return quantities: %w", err)
		}
		for returnedRows.Next() {
			var poItemID string
			var qty float64
			if err := returnedRows.Scan(&poItemID, &qty); err != nil {
				returnedRows.Close()
				return "", err
			}
			if entry, ok := qtyMap[poItemID]; ok {
				entry.returned = qty
			}
		}
		returnedRows.Close()

		// check each incoming item
		for _, it := range in.Items {
			if it.PurchaseOrderItemID == "" {
				continue // can't track items without a PO item link
			}
			entry, ok := qtyMap[it.PurchaseOrderItemID]
			if !ok {
				return "", fmt.Errorf("item %q does not belong to invoice %q", it.ItemName, in.InvoiceNumber)
			}
			remaining := entry.invoiced - entry.returned
			if remaining <= 0 {
				return "", fmt.Errorf("item %q has already been fully returned", it.ItemName)
			}
			if it.Quantity > remaining {
				return "", fmt.Errorf("item %q: return quantity %.2f exceeds remaining returnable quantity %.2f", it.ItemName, it.Quantity, remaining)
			}
		}
	}

	// Calculate totals
	var subAmount, totalGST float64
	type calcItem struct {
		poItemID     string
		itemName     string
		hsnCode      string
		unit         string
		qty          float64
		unitPrice    float64
		gstPercent   float64
		gstAmount    float64
		totalAmount  float64
		reason       string
		taxInclusive bool
	}
	var items []calcItem

	for _, it := range in.Items {
		if it.Quantity <= 0 {
			return "", fmt.Errorf("quantity for item %q must be > 0", it.ItemName)
		}
		var linePrice, gstAmt float64
		if it.TaxInclusive {
			// unit_price already includes GST
			gstAmt = it.Quantity * it.UnitPrice * it.GSTPercent / (100 + it.GSTPercent)
			linePrice = it.Quantity*it.UnitPrice - gstAmt
		} else {
			linePrice = it.Quantity * it.UnitPrice
			gstAmt = linePrice * it.GSTPercent / 100
		}
		total := linePrice + gstAmt
		subAmount += linePrice
		totalGST += gstAmt

		items = append(items, calcItem{
			poItemID:     it.PurchaseOrderItemID,
			itemName:     it.ItemName,
			hsnCode:      it.HSNCode,
			unit:         it.Unit,
			qty:          it.Quantity,
			unitPrice:    it.UnitPrice,
			gstPercent:   it.GSTPercent,
			gstAmount:    gstAmt,
			totalAmount:  total,
			reason:       it.Reason,
			taxInclusive: it.TaxInclusive,
		})
	}

	netAmount := subAmount + totalGST + in.DutyAmount + in.RoundOff

	// Insert purchase_return header
	var prID string
	err = tx.QueryRow(`
		INSERT INTO purchase_returns
			(pr_number, pr_date, supplier_id, purchase_invoice_id, goods_receipt_id,
			 currency, exchange_rate, sub_amount, tax_amount, duty_amount,
			 round_off, net_amount, reason, status, created_by)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
		        $6, $7, $8, $9, $10, $11, $12, $13, 'PENDING', $14)
		RETURNING id
	`, prNumber, prDate, in.SupplierID, purchaseInvoiceID, grnID,
		currency, exchangeRate, subAmount, totalGST, in.DutyAmount,
		in.RoundOff, netAmount, in.Reason, userID).Scan(&prID)
	if err != nil {
		return "", fmt.Errorf("insert purchase_returns: %w", err)
	}

	// Insert items + reverse stock
	for _, it := range items {
		_, err = tx.Exec(`
			INSERT INTO purchase_return_items
				(id, purchase_return_id, purchase_order_item_id, item_name, hsn_code,
				 unit, quantity, unit_price, gst_percent, gst_amount, total_amount,
				 reason, tax_inclusive)
			VALUES ($1, $2, NULLIF($3,'')::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, uuid.New().String(), prID, it.poItemID, it.itemName, it.hsnCode,
			it.unit, it.qty, it.unitPrice, it.gstPercent, it.gstAmount, it.totalAmount,
			it.reason, it.taxInclusive)
		if err != nil {
			return "", fmt.Errorf("insert purchase_return_items: %w", err)
		}

		if warehouseID != "" {
			// Deduct from raw_material_stocks
			_, err = tx.Exec(`
				UPDATE raw_material_stocks
				SET quantity = quantity - $1, updated_at = NOW()
				WHERE item_name = $2 AND warehouse_id = $3
			`, it.qty, it.itemName, warehouseID)
			if err != nil {
				return "", fmt.Errorf("deduct raw_material_stocks: %w", err)
			}

			// Record movement (OUT type for return, negative quantity)
			_, err = tx.Exec(`
				INSERT INTO raw_material_movements
					(item_name, warehouse_id, quantity, movement_type, reference)
				VALUES ($1, $2, $3, 'OUT', $4)
			`, it.itemName, warehouseID, it.qty, prNumber)
			if err != nil {
				return "", fmt.Errorf("insert raw_material_movements: %w", err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}
	return prID, nil
}

// List returns purchase returns with optional filters.
func (s *Store) List(f ListFilter) ([]PurchaseReturnListRow, error) {
	base := `
		SELECT
			pr.id, pr.pr_number, pr.pr_date::text,
			pr.supplier_id, COALESCE(sup.name, ''),
			COALESCE(pr.goods_receipt_id::text, ''), COALESCE(gr.grn_number, ''),
			COALESCE(pr.purchase_invoice_id::text, ''), COALESCE(pi.invoice_number, ''),
			pr.sub_amount, pr.tax_amount, pr.net_amount, pr.status
		FROM purchase_returns pr
		LEFT JOIN suppliers sup        ON sup.id = pr.supplier_id
		LEFT JOIN goods_receipts gr    ON gr.id  = pr.goods_receipt_id
		LEFT JOIN purchase_invoices pi ON pi.id  = pr.purchase_invoice_id
	`

	var conditions []string
	var args []interface{}
	idx := 1

	if f.SupplierName != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(sup.name) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.SupplierName)+"%")
		idx++
	}
	if f.PRNumber != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(pr.pr_number) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.PRNumber)+"%")
		idx++
	}
	if f.GRNNumber != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(gr.grn_number) LIKE $%d", idx))
		args = append(args, "%"+strings.ToLower(f.GRNNumber)+"%")
		idx++
	}
	if f.DateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("pr.pr_date >= $%d", idx))
		args = append(args, f.DateFrom)
		idx++
	}
	if f.DateTo != "" {
		conditions = append(conditions, fmt.Sprintf("pr.pr_date <= $%d", idx))
		args = append(args, f.DateTo)
		idx++
	}

	if len(conditions) > 0 {
		base += " WHERE " + strings.Join(conditions, " AND ")
	}
	base += " ORDER BY pr.pr_date DESC, pr.created_at DESC"

	rows, err := s.db.Query(base, args...)
	if err != nil {
		return nil, fmt.Errorf("list purchase_returns: %w", err)
	}
	defer rows.Close()

	var list []PurchaseReturnListRow
	for rows.Next() {
		var r PurchaseReturnListRow
		if err := rows.Scan(
			&r.ID, &r.PRNumber, &r.PRDate,
			&r.SupplierID, &r.SupplierName,
			&r.GoodsReceiptID, &r.GRNNumber,
			&r.PurchaseInvoiceID, &r.InvoiceNumber,
			&r.SubAmount, &r.TaxAmount, &r.NetAmount, &r.Status,
		); err != nil {
			return nil, fmt.Errorf("scan purchase_return row: %w", err)
		}
		list = append(list, r)
	}
	if list == nil {
		list = []PurchaseReturnListRow{}
	}
	return list, nil
}

// GetByID returns full detail for one purchase return.
func (s *Store) GetByID(id string) (*PurchaseReturnDetail, error) {
	var d PurchaseReturnDetail
	var reason sql.NullString
	var piID, piNumber, grnID, grnNumber sql.NullString

	err := s.db.QueryRow(`
		SELECT
			pr.id, pr.pr_number, pr.pr_date::text,
			pr.supplier_id, COALESCE(sup.name, ''),
			pr.purchase_invoice_id, COALESCE(pi.invoice_number, ''),
			pr.goods_receipt_id, COALESCE(gr.grn_number, ''),
			pr.currency, pr.exchange_rate,
			pr.sub_amount, pr.tax_amount, pr.duty_amount, pr.round_off, pr.net_amount,
			pr.reason, pr.status, pr.created_at::text
		FROM purchase_returns pr
		LEFT JOIN suppliers sup        ON sup.id = pr.supplier_id
		LEFT JOIN purchase_invoices pi ON pi.id  = pr.purchase_invoice_id
		LEFT JOIN goods_receipts gr    ON gr.id  = pr.goods_receipt_id
		WHERE pr.id = $1
	`, id).Scan(
		&d.ID, &d.PRNumber, &d.PRDate,
		&d.SupplierID, &d.SupplierName,
		&piID, &piNumber,
		&grnID, &grnNumber,
		&d.Currency, &d.ExchangeRate,
		&d.SubAmount, &d.TaxAmount, &d.DutyAmount, &d.RoundOff, &d.NetAmount,
		&reason, &d.Status, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if piID.Valid {
		d.PurchaseInvoiceID = piID.String
	}
	if piNumber.Valid {
		d.InvoiceNumber = piNumber.String
	}
	if grnID.Valid {
		d.GoodsReceiptID = grnID.String
	}
	if grnNumber.Valid {
		d.GRNNumber = grnNumber.String
	}
	if reason.Valid {
		d.Reason = reason.String
	}

	// Items
	rows, err := s.db.Query(`
		SELECT
			id,
			COALESCE(purchase_order_item_id::text, ''),
			item_name, COALESCE(hsn_code, ''), COALESCE(unit, ''),
			quantity, unit_price, gst_percent, gst_amount, total_amount,
			COALESCE(reason, ''), tax_inclusive
		FROM purchase_return_items
		WHERE purchase_return_id = $1
		ORDER BY id
	`, id)
	if err != nil {
		return nil, fmt.Errorf("fetch purchase_return_items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it PurchaseReturnDetailItem
		if err := rows.Scan(
			&it.ID, &it.PurchaseOrderItemID,
			&it.ItemName, &it.HSNCode, &it.Unit,
			&it.Quantity, &it.UnitPrice, &it.GSTPercent, &it.GSTAmount, &it.TotalAmount,
			&it.Reason, &it.TaxInclusive,
		); err != nil {
			return nil, fmt.Errorf("scan purchase_return_item: %w", err)
		}
		d.Items = append(d.Items, it)
	}
	if d.Items == nil {
		d.Items = []PurchaseReturnDetailItem{}
	}
	return &d, nil
}

// GetInvoiceLookup returns pre-populated purchase return form data from an invoice number.
func (s *Store) GetInvoiceLookup(invoiceNumber string) (*InvoiceLookupResponse, error) {
	var resp InvoiceLookupResponse
	resp.Currency = "Rs"
	resp.ExchangeRate = 1

	err := s.db.QueryRow(`
		SELECT pi.invoice_number, pi.supplier_id, COALESCE(sup.name, '')
		FROM purchase_invoices pi
		LEFT JOIN suppliers sup ON sup.id = pi.supplier_id
		WHERE pi.invoice_number = $1
	`, invoiceNumber).Scan(&resp.InvoiceNumber, &resp.SupplierID, &resp.SupplierName)
	if err != nil {
		return nil, fmt.Errorf("invoice %q not found", invoiceNumber)
	}

	rows, err := s.db.Query(`
		SELECT
			pii.purchase_order_item_id::text,
			pii.item_name,
			COALESCE(pii.hsn_code, ''),
			COALESCE(pii.unit, ''),
			pii.quantity,
			pii.unit_price,
			pii.tax_percent
		FROM purchase_invoice_items pii
		JOIN purchase_invoices pi ON pi.id = pii.purchase_invoice_id
		WHERE pi.invoice_number = $1
		ORDER BY pii.id
	`, invoiceNumber)
	if err != nil {
		return nil, fmt.Errorf("fetch invoice items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it InvoiceLookupItem
		if err := rows.Scan(
			&it.PurchaseOrderItemID,
			&it.ItemName, &it.HSNCode, &it.Unit,
			&it.Quantity, &it.UnitPrice, &it.GSTPercent,
		); err != nil {
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		resp.Items = append(resp.Items, it)
	}
	if resp.Items == nil {
		resp.Items = []InvoiceLookupItem{}
	}
	return &resp, nil
}

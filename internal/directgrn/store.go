package directgrn

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

// poItemRecord holds the data needed for GRN + invoice creation after PO items are inserted.
type poItemRecord struct {
	id          string
	itemName    string
	productCode string
	hsnCode     string
	unit        string
	qty         float64
	unitPrice   float64
	gstPercent  float64
}

// Create runs a single DB transaction that creates a Purchase Order, Goods Receipt,
// and Purchase Invoice together (Direct GRN workflow).
func (s *Store) Create(in DirectGRNInput, userID string) (*DirectGRNResult, error) {
	if len(in.Items) == 0 {
		return nil, errors.New("at least one item is required")
	}
	if in.SupplierID == "" {
		return nil, errors.New("supplier_id is required")
	}
	if in.WarehouseID == "" {
		return nil, errors.New("warehouse_id is required")
	}
	if in.InvoiceDate == "" {
		return nil, errors.New("invoice_date is required")
	}

	orderDate := in.OrderDate
	if orderDate == "" {
		orderDate = time.Now().Format("2006-01-02")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// ─── 1. Create Purchase Order ───────────────────────────────────────────
	poID := uuid.New().String()
	poNumber := "PO-" + time.Now().Format("20060102150405")

	_, err = tx.Exec(`
		INSERT INTO purchase_orders
			(id, po_number, supplier_id, warehouse_id, status, order_date, purchase_type, created_at)
		VALUES ($1, $2, $3, $4, 'DRAFT', $5, $6, NOW())
	`, poID, poNumber, in.SupplierID, in.WarehouseID, orderDate, in.PurchaseType)
	if err != nil {
		return nil, fmt.Errorf("insert purchase_orders: %w", err)
	}

	// ─── 2. Create Purchase Order Items ─────────────────────────────────────
	var totalAmount, taxAmount float64
	var poItems []poItemRecord

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("quantity for item %q must be greater than zero", item.ItemName)
		}
		itemID := uuid.New().String()
		var gstAmt, lineBase, lineTotal float64
		if in.TaxInclusive {
			// unit_price includes GST — back-calculate
			gstAmt = item.Quantity * item.UnitPrice * item.GSTPercent / (100 + item.GSTPercent)
			lineBase = item.Quantity*item.UnitPrice - gstAmt
			lineTotal = item.Quantity * item.UnitPrice
		} else {
			lineBase = item.Quantity * item.UnitPrice
			gstAmt = lineBase * item.GSTPercent / 100
			lineTotal = lineBase + gstAmt
		}
		totalAmount += lineBase
		taxAmount += gstAmt

		_, err = tx.Exec(`
			INSERT INTO purchase_order_items
				(id, purchase_order_id, item_name, description, hsn_code, unit,
				 quantity, unit_price, gst_percent, gst_amount, total_price,
				 product_code, category, free_qty,
				 additional_work, additional_work_amount,
				 paid_by_user_id, paid_to_supplier_id, cash_amount, credit_amount,
				 tax_inclusive)
			VALUES
				($1, $2, $3, $4, $5, $6,
				 $7, $8, $9, $10, $11,
				 $12, $13, $14,
				 $15, $16,
				 NULLIF($17,'')::uuid, NULLIF($18,'')::uuid, $19, $20,
				 $21)
		`,
			itemID, poID, item.ItemName, item.Description, item.HSNCode, item.Unit,
			item.Quantity, item.UnitPrice, item.GSTPercent, gstAmt, lineTotal,
			item.ProductCode, item.Category, item.FreeQty,
			item.AdditionalWork, item.AdditionalWorkAmount,
			item.PaidByUserID, item.PaidToSupplierID, item.CashAmount, item.CreditAmount,
			in.TaxInclusive,
		)
		if err != nil {
			return nil, fmt.Errorf("insert purchase_order_items: %w", err)
		}

		poItems = append(poItems, poItemRecord{
			id:          itemID,
			itemName:    item.ItemName,
			productCode: item.ProductCode,
			hsnCode:     item.HSNCode,
			unit:        item.Unit,
			qty:         item.Quantity,
			unitPrice:   item.UnitPrice,
			gstPercent:  item.GSTPercent,
		})
	}

	grandTotal := totalAmount + taxAmount
	_, err = tx.Exec(`
		UPDATE purchase_orders SET total_amount = $1, tax_amount = $2, grand_total = $3 WHERE id = $4
	`, totalAmount, taxAmount, grandTotal, poID)
	if err != nil {
		return nil, fmt.Errorf("update purchase_order totals: %w", err)
	}

	// ─── 3. Create Purchase Charges ─────────────────────────────────────────
	for _, ch := range in.Charges {
		if ch.Amount <= 0 || ch.ChargeType == "" {
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO purchase_charges (id, purchase_order_id, charge_type, amount, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, uuid.New().String(), poID, ch.ChargeType, ch.Amount)
		if err != nil {
			return nil, fmt.Errorf("insert purchase_charges: %w", err)
		}
	}

	// ─── 4. Generate GRN Number ─────────────────────────────────────────────
	var grnCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM goods_receipts`).Scan(&grnCount); err != nil {
		return nil, fmt.Errorf("count goods_receipts: %w", err)
	}
	grnNumber := fmt.Sprintf("GRN-%03d", grnCount+1)

	// ─── 5. Create Goods Receipt ────────────────────────────────────────────
	var grnID string
	err = tx.QueryRow(`
		INSERT INTO goods_receipts
			(grn_number, purchase_order_id, supplier_id, warehouse_id,
			 received_by, received_date, reference, transport_supplier_id, lr_number, status)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, NULLIF($7,'')::uuid, $8, 'COMPLETED')
		RETURNING id
	`, grnNumber, poID, in.SupplierID, in.WarehouseID,
		userID, in.Reference, in.TransportSupplierID, in.LRNumber).Scan(&grnID)
	if err != nil {
		return nil, fmt.Errorf("insert goods_receipts: %w", err)
	}

	// ─── 6. Create GRN Items + Update Stock ─────────────────────────────────
	for _, it := range poItems {
		_, err = tx.Exec(`
			INSERT INTO goods_receipt_items
				(goods_receipt_id, purchase_order_item_id, ordered_qty, received_qty)
			VALUES ($1, $2, $3, $4)
		`, grnID, it.id, it.qty, it.qty)
		if err != nil {
			return nil, fmt.Errorf("insert goods_receipt_items: %w", err)
		}

		_, err = tx.Exec(`
			UPDATE purchase_order_items SET received_qty = received_qty + $1 WHERE id = $2
		`, it.qty, it.id)
		if err != nil {
			return nil, fmt.Errorf("update po item received_qty: %w", err)
		}

		// Upsert raw_material_stocks — conflict key is (product_code, warehouse_id) when
		// product_code is provided, otherwise falls back to (item_name, warehouse_id).
		if it.productCode != "" {
			_, err = tx.Exec(`
				INSERT INTO raw_material_stocks (item_name, product_code, hsn_code, unit, warehouse_id, quantity, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, NOW())
				ON CONFLICT (product_code, warehouse_id) WHERE product_code IS NOT NULL AND product_code <> ''
				DO UPDATE SET
					quantity     = raw_material_stocks.quantity + $6,
					item_name    = $1,
					hsn_code     = COALESCE(NULLIF($3, ''), raw_material_stocks.hsn_code),
					unit         = $4,
					updated_at   = NOW()
			`, it.itemName, it.productCode, it.hsnCode, it.unit, in.WarehouseID, it.qty)
		} else {
			_, err = tx.Exec(`
				INSERT INTO raw_material_stocks (item_name, hsn_code, unit, warehouse_id, quantity, updated_at)
				VALUES ($1, $2, $3, $4, $5, NOW())
				ON CONFLICT (item_name, warehouse_id) WHERE product_code IS NULL OR product_code = ''
				DO UPDATE SET
					quantity   = raw_material_stocks.quantity + $5,
					hsn_code   = COALESCE(NULLIF($2, ''), raw_material_stocks.hsn_code),
					unit       = $3,
					updated_at = NOW()
			`, it.itemName, it.hsnCode, it.unit, in.WarehouseID, it.qty)
		}
		if err != nil {
			return nil, fmt.Errorf("upsert raw_material_stocks: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO raw_material_movements
				(item_name, warehouse_id, quantity, movement_type, goods_receipt_id, purchase_order_id, reference)
			VALUES ($1, $2, $3, 'IN', $4, $5, $6)
		`, it.itemName, in.WarehouseID, it.qty, grnID, poID, in.Reference)
		if err != nil {
			return nil, fmt.Errorf("insert raw_material_movements: %w", err)
		}
	}

	// ─── 7. Mark PO as RECEIVED ─────────────────────────────────────────────
	_, err = tx.Exec(`UPDATE purchase_orders SET status = 'RECEIVED' WHERE id = $1`, poID)
	if err != nil {
		return nil, fmt.Errorf("update purchase_order status: %w", err)
	}

	// ─── 8. Generate Invoice Number ─────────────────────────────────────────
	invoiceNumber := in.InvoiceNumber
	if invoiceNumber == "" {
		var invCount int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM purchase_invoices`).Scan(&invCount); err != nil {
			return nil, fmt.Errorf("count purchase_invoices: %w", err)
		}
		invoiceNumber = fmt.Sprintf("PI-%03d", invCount+1)
	}

	// ─── 9. Create Purchase Invoice ─────────────────────────────────────────
	subAmount := totalAmount - in.DiscountAmount
	netAmount := subAmount + taxAmount + in.RoundOff

	var invoiceID string
	err = tx.QueryRow(`
		INSERT INTO purchase_invoices
			(invoice_number, purchase_order_id, supplier_id, warehouse_id,
			 invoice_date, sub_amount, discount_amount, gst_amount, round_off,
			 net_amount, paid_amount, status, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0, 'PENDING', $11, $12)
		RETURNING id
	`, invoiceNumber, poID, in.SupplierID, in.WarehouseID,
		in.InvoiceDate, subAmount, in.DiscountAmount, taxAmount, in.RoundOff,
		netAmount, in.Notes, userID).Scan(&invoiceID)
	if err != nil {
		return nil, fmt.Errorf("insert purchase_invoices: %w", err)
	}

	// ─── 10. Create Purchase Invoice Items ──────────────────────────────────
	for _, it := range poItems {
		gstAmt := it.qty * it.unitPrice * it.gstPercent / 100
		lineTotal := (it.qty * it.unitPrice) + gstAmt

		_, err = tx.Exec(`
			INSERT INTO purchase_invoice_items
				(purchase_invoice_id, purchase_order_item_id, item_name, hsn_code,
				 unit, quantity, unit_price, tax_percent, tax_amount, total_amount)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, invoiceID, it.id, it.itemName, it.hsnCode,
			it.unit, it.qty, it.unitPrice, it.gstPercent, gstAmt, lineTotal)
		if err != nil {
			return nil, fmt.Errorf("insert purchase_invoice_items: %w", err)
		}
	}

	// ─── 11. Record Payment (optional) ──────────────────────────────────────
	if in.PaymentAmount > 0 && in.PaymentMethod != "" {
		if in.PaymentAmount > netAmount {
			return nil, fmt.Errorf("payment %.2f exceeds invoice amount %.2f", in.PaymentAmount, netAmount)
		}
		_, err = tx.Exec(`
			INSERT INTO supplier_payments (purchase_invoice_id, amount, payment_method, reference, paid_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, invoiceID, in.PaymentAmount, in.PaymentMethod, in.Reference)
		if err != nil {
			return nil, fmt.Errorf("insert supplier_payments: %w", err)
		}

		payStatus := "PARTIALLY_PAID"
		if in.PaymentAmount >= netAmount {
			payStatus = "PAID"
		}
		_, err = tx.Exec(`
			UPDATE purchase_invoices SET paid_amount = $1, status = $2 WHERE id = $3
		`, in.PaymentAmount, payStatus, invoiceID)
		if err != nil {
			return nil, fmt.Errorf("update invoice payment status: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &DirectGRNResult{
		POID:          poID,
		PONumber:      poNumber,
		GRNID:         grnID,
		GRNNumber:     grnNumber,
		InvoiceID:     invoiceID,
		InvoiceNumber: invoiceNumber,
		NetAmount:     netAmount,
	}, nil
}

// ListResult wraps paginated rows and total count.
type ListResult struct {
	Data       []DirectGRNListRow `json:"data"`
	Total      int                `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalPages int                `json:"total_pages"`
}

// List returns Direct GRNs with optional filters and pagination.
func (s *Store) List(f ListFilter) (*ListResult, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	base := `
		SELECT
			gr.id                                     AS grn_id,
			gr.grn_number,
			po.id                                     AS po_id,
			po.po_number,
			COALESCE(po.purchase_type, '')            AS purchase_type,
			po.supplier_id,
			COALESCE(sup.name, '')                    AS supplier_name,
			po.warehouse_id,
			COALESCE(w.name, '')                      AS warehouse_name,
			COALESCE(gr.transport_supplier_id::text, '') AS transport_supplier_id,
			COALESCE(tsup.name, '')                  AS transport_supplier_name,
			COALESCE(gr.lr_number, '')               AS lr_number,
			gr.received_date::text                   AS received_date,
			COALESCE(po.expected_date::text, '')     AS expected_date,
			pi.id                                    AS invoice_id,
			pi.invoice_number,
			pi.invoice_date::text                    AS invoice_date,
			pi.net_amount,
			COALESCE(
				(SELECT SUM(pc.amount) FROM purchase_charges pc WHERE pc.purchase_order_id = po.id),
				0
			)                                        AS additional_charges,
			COALESCE(
				(SELECT SUM(poi.additional_work_amount) FROM purchase_order_items poi WHERE poi.purchase_order_id = po.id),
				0
			)                                        AS additional_work_amount
		FROM goods_receipts gr
		JOIN purchase_orders po      ON po.id = gr.purchase_order_id
		JOIN purchase_invoices pi    ON pi.purchase_order_id = po.id
		LEFT JOIN suppliers sup      ON sup.id = po.supplier_id
		LEFT JOIN warehouses w       ON w.id = po.warehouse_id
		LEFT JOIN suppliers tsup     ON tsup.id = gr.transport_supplier_id
	`

	var conditions []string
	var args []interface{}
	argIdx := 1

	if f.GRNNumber != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(gr.grn_number) LIKE $%d", argIdx))
		args = append(args, "%"+strings.ToLower(f.GRNNumber)+"%")
		argIdx++
	}
	if f.SupplierName != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(sup.name) LIKE $%d", argIdx))
		args = append(args, "%"+strings.ToLower(f.SupplierName)+"%")
		argIdx++
	}
	if f.DateFrom != "" {
		conditions = append(conditions, fmt.Sprintf("gr.received_date >= $%d", argIdx))
		args = append(args, f.DateFrom)
		argIdx++
	}
	if f.DateTo != "" {
		conditions = append(conditions, fmt.Sprintf("gr.received_date <= $%d", argIdx))
		args = append(args, f.DateTo+" 23:59:59")
		argIdx++
	}

	if len(conditions) > 0 {
		base += " WHERE " + strings.Join(conditions, " AND ")
	}

	// total count (same filters, no pagination)
	countQ := "SELECT COUNT(*) FROM (" + base + ") AS _cnt"
	var total int
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count direct grn: %w", err)
	}

	base += fmt.Sprintf(" ORDER BY gr.received_date DESC LIMIT %d OFFSET %d", limit, offset)

	rows, err := s.db.Query(base, args...)
	if err != nil {
		return nil, fmt.Errorf("list direct grn: %w", err)
	}
	defer rows.Close()

	var list []DirectGRNListRow
	for rows.Next() {
		var r DirectGRNListRow
		if err := rows.Scan(
			&r.GRNID, &r.GRNNumber,
			&r.POID, &r.PONumber, &r.PurchaseType,
			&r.SupplierID, &r.SupplierName,
			&r.WarehouseID, &r.WarehouseName,
			&r.TransportSupplierID, &r.TransportSupplierName, &r.LRNumber,
			&r.ReceivedDate, &r.ExpectedDate,
			&r.InvoiceID, &r.InvoiceNumber, &r.InvoiceDate,
			&r.NetAmount, &r.AdditionalCharges, &r.AdditionalWorkAmount,
		); err != nil {
			return nil, fmt.Errorf("scan direct grn list row: %w", err)
		}
		list = append(list, r)
	}
	if list == nil {
		list = []DirectGRNListRow{}
	}

	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	return &ListResult{
		Data:       list,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// GetByID returns full detail for one Direct GRN by GRN id.
func (s *Store) GetByID(grnID string) (*DirectGRNDetail, error) {
	var d DirectGRNDetail
	var notes, reference sql.NullString

	err := s.db.QueryRow(`
		SELECT
			gr.id, gr.grn_number,
			po.id, po.po_number, COALESCE(po.purchase_type, ''),
			po.supplier_id, COALESCE(sup.name, ''),
			po.warehouse_id, COALESCE(w.name, ''),
			COALESCE(gr.transport_supplier_id::text, ''), COALESCE(tsup.name, ''),
			COALESCE(gr.lr_number, ''),
			gr.received_date::text,
			COALESCE(po.expected_date::text, ''),
			pi.id, pi.invoice_number, pi.invoice_date::text,
			pi.sub_amount, pi.discount_amount, pi.gst_amount, pi.round_off,
			pi.net_amount, pi.paid_amount, pi.status,
			pi.notes, gr.reference
		FROM goods_receipts gr
		JOIN purchase_orders po      ON po.id = gr.purchase_order_id
		JOIN purchase_invoices pi    ON pi.purchase_order_id = po.id
		LEFT JOIN suppliers sup      ON sup.id = po.supplier_id
		LEFT JOIN warehouses w       ON w.id = po.warehouse_id
		LEFT JOIN suppliers tsup     ON tsup.id = gr.transport_supplier_id
		WHERE gr.id = $1
	`, grnID).Scan(
		&d.GRNID, &d.GRNNumber,
		&d.POID, &d.PONumber, &d.PurchaseType,
		&d.SupplierID, &d.SupplierName,
		&d.WarehouseID, &d.WarehouseName,
		&d.TransportSupplierID, &d.TransportSupplierName, &d.LRNumber,
		&d.ReceivedDate, &d.ExpectedDate,
		&d.InvoiceID, &d.InvoiceNumber, &d.InvoiceDate,
		&d.SubAmount, &d.DiscountAmount, &d.GSTAmount, &d.RoundOff,
		&d.NetAmount, &d.PaidAmount, &d.Status,
		&notes, &reference,
	)
	if err != nil {
		return nil, err
	}
	if notes.Valid {
		d.Notes = notes.String
	}
	if reference.Valid {
		d.Reference = reference.String
	}

	// Items
	itemRows, err := s.db.Query(`
		SELECT
			poi.id, poi.item_name, COALESCE(poi.description, ''),
			COALESCE(poi.product_code, ''), COALESCE(poi.category, ''),
			COALESCE(poi.hsn_code, ''), COALESCE(poi.unit, ''),
			poi.quantity, poi.free_qty, poi.unit_price,
			poi.gst_percent, poi.gst_amount, poi.total_price,
			COALESCE(poi.additional_work, ''), poi.additional_work_amount,
			COALESCE(poi.paid_by_user_id::text, ''), COALESCE(u.name, ''),
			COALESCE(poi.paid_to_supplier_id::text, ''), COALESCE(sp.name, ''),
			poi.cash_amount, poi.credit_amount,
			COALESCE(poi.tax_inclusive, FALSE)
		FROM purchase_order_items poi
		LEFT JOIN users u      ON u.id  = poi.paid_by_user_id
		LEFT JOIN suppliers sp ON sp.id = poi.paid_to_supplier_id
		WHERE poi.purchase_order_id = $1
		ORDER BY poi.id
	`, d.POID)
	if err != nil {
		return nil, fmt.Errorf("fetch items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var it DirectGRNDetailItem
		if err := itemRows.Scan(
			&it.ID, &it.ItemName, &it.Description,
			&it.ProductCode, &it.Category,
			&it.HSNCode, &it.Unit,
			&it.Quantity, &it.FreeQty, &it.UnitPrice,
			&it.GSTPercent, &it.GSTAmount, &it.TotalPrice,
			&it.AdditionalWork, &it.AdditionalWorkAmount,
			&it.PaidByUserID, &it.PaidByUserName,
			&it.PaidToSupplierID, &it.PaidToSupplierName,
			&it.CashAmount, &it.CreditAmount,
			&it.TaxInclusive,
		); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		d.Items = append(d.Items, it)
	}
	if d.Items == nil {
		d.Items = []DirectGRNDetailItem{}
	}

	// Charges
	chargeRows, err := s.db.Query(`
		SELECT id, charge_type, amount FROM purchase_charges WHERE purchase_order_id = $1 ORDER BY created_at
	`, d.POID)
	if err != nil {
		return nil, fmt.Errorf("fetch charges: %w", err)
	}
	defer chargeRows.Close()

	for chargeRows.Next() {
		var ch DirectGRNDetailCharge
		if err := chargeRows.Scan(&ch.ID, &ch.ChargeType, &ch.Amount); err != nil {
			return nil, fmt.Errorf("scan charge: %w", err)
		}
		d.Charges = append(d.Charges, ch)
	}
	if d.Charges == nil {
		d.Charges = []DirectGRNDetailCharge{}
	}

	return &d, nil
}

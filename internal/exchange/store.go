package exchange

import (
	"database/sql"
	"fmt"
	"math"
)

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ─────────────────────────────────────────────────────────────────────────────
// Create processes an exchange atomically in one transaction:
//  1. Validates original invoice and items_out
//  2. Calculates amounts for items_out (credit note value)
//  3. Resolves prices/tax for items_in (new invoice value)
//  4. Creates a return_order (credit note, source='EXCHANGE') – original invoice untouched
//  5. Creates a new sales_order + sales_invoice (channel='EXCHANGE')
//  6. Adjusts stock in both directions
//  7. Records settlements and exchange_order header
//
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) Create(in CreateExchangeInput, userID, branchID string) (string, error) {
	if in.OriginalSalesInvoiceID == "" {
		return "", fmt.Errorf("original_sales_invoice_id is required")
	}
	if len(in.ItemsOut) == 0 {
		return "", fmt.Errorf("items_out cannot be empty")
	}
	if len(in.ItemsIn) == 0 {
		return "", fmt.Errorf("items_in cannot be empty")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// ── 1. Load original invoice ─────────────────────────────────────────────
	var inv struct {
		ID             string
		CustomerID     string
		BranchID       sql.NullString
		WarehouseID    string
		Status         string
		SubAmount      float64
		DiscountAmount float64
		BillDiscount   float64
		GSTAmount      float64
		NetAmount      float64
		SalespersonID  sql.NullString
	}
	if err := tx.QueryRow(`
		SELECT si.id, si.customer_id, si.branch_id, si.warehouse_id, si.status,
		       si.sub_amount, si.discount_amount, si.bill_discount, si.gst_amount, si.net_amount,
		       so.salesperson_id
		FROM sales_invoices si
		LEFT JOIN sales_orders so ON so.id = si.sales_order_id
		WHERE si.id = $1
	`, in.OriginalSalesInvoiceID).Scan(
		&inv.ID, &inv.CustomerID, &inv.BranchID, &inv.WarehouseID, &inv.Status,
		&inv.SubAmount, &inv.DiscountAmount, &inv.BillDiscount, &inv.GSTAmount, &inv.NetAmount,
		&inv.SalespersonID,
	); err != nil {
		return "", fmt.Errorf("original invoice not found: %w", err)
	}
	if inv.Status == "CANCELLED" {
		return "", fmt.Errorf("cannot exchange against a cancelled invoice")
	}

	// ── 2. Generate exchange number and insert header (placeholder totals) ───
	exchangeNumber := s.nextExchangeNumber(tx)
	var exchangeID string
	if err := tx.QueryRow(`
		INSERT INTO exchange_orders
			(exchange_number, original_sales_invoice_id, branch_id, warehouse_id,
			 customer_id, status, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, 'COMPLETED', $6, $7)
		RETURNING id
	`, exchangeNumber, inv.ID, inv.BranchID, inv.WarehouseID,
		inv.CustomerID, in.Notes, userID).Scan(&exchangeID); err != nil {
		return "", fmt.Errorf("create exchange order: %w", err)
	}

	// ── 3. Process items_out ─────────────────────────────────────────────────
	type outCalc struct {
		variantID      string
		salesInvItemID string
		quantity       float64
		unitPrice      float64
		discount       float64
		billDiscShare  float64
		taxPercent     float64
		taxAmount      float64
		totalPrice     float64
		reason         string
	}
	var outsCalc []outCalc
	var totalOutAmount, totalGSTOut float64

	for _, item := range in.ItemsOut {
		if item.Quantity <= 0 {
			return "", fmt.Errorf("items_out quantity must be > 0")
		}

		var si struct {
			ID         string
			VariantID  string
			Quantity   float64
			UnitPrice  float64
			Discount   float64
			TaxPercent float64
		}
		if err := tx.QueryRow(`
			SELECT id, variant_id, quantity, unit_price, discount, tax_percent
			FROM sales_invoice_items
			WHERE id = $1 AND sales_invoice_id = $2
		`, item.SalesInvoiceItemID, inv.ID).Scan(
			&si.ID, &si.VariantID, &si.Quantity,
			&si.UnitPrice, &si.Discount, &si.TaxPercent,
		); err != nil {
			return "", fmt.Errorf("invoice item %s not found on invoice: %w", item.SalesInvoiceItemID, err)
		}
		if item.Quantity > si.Quantity {
			return "", fmt.Errorf("exchange qty %.2f exceeds invoiced qty %.2f for item %s",
				item.Quantity, si.Quantity, si.ID)
		}

		// Check already exchanged/returned quantity (covers both returns and exchange credit notes)
		var alreadyProcessed float64
		if err := tx.QueryRow(`
			SELECT COALESCE(SUM(ri.quantity), 0)
			FROM return_items ri
			JOIN return_orders ro ON ri.return_order_id = ro.id
			WHERE ro.sales_invoice_id = $1
			  AND ri.sales_invoice_item_id = $2
			  AND ro.status != 'CANCELLED'
		`, inv.ID, si.ID).Scan(&alreadyProcessed); err != nil {
			return "", fmt.Errorf("qty check for item %s: %w", si.ID, err)
		}
		if alreadyProcessed+item.Quantity > si.Quantity {
			return "", fmt.Errorf("cannot exchange %.2f of item %s: only %.2f remaining (%.2f already processed)",
				item.Quantity, si.ID, si.Quantity-alreadyProcessed, alreadyProcessed)
		}

		lineTotal := item.Quantity * si.UnitPrice
		itemDisc := round2(si.Discount * item.Quantity / si.Quantity)
		lineBillDisc := 0.0
		if inv.SubAmount > 0 {
			lineBillDisc = round2(lineTotal * inv.BillDiscount / inv.SubAmount)
		}
		taxable := lineTotal - itemDisc - lineBillDisc
		if taxable < 0 {
			taxable = 0
		}
		lineTax := round2(taxable * si.TaxPercent / (100 + si.TaxPercent))
		lineReturn := round2(lineTotal - itemDisc - lineBillDisc)

		oc := outCalc{
			variantID:      si.VariantID,
			salesInvItemID: si.ID,
			quantity:       item.Quantity,
			unitPrice:      si.UnitPrice,
			discount:       itemDisc,
			billDiscShare:  lineBillDisc,
			taxPercent:     si.TaxPercent,
			taxAmount:      lineTax,
			totalPrice:     lineReturn,
			reason:         item.Reason,
		}
		outsCalc = append(outsCalc, oc)

		if _, err := tx.Exec(`
			INSERT INTO exchange_items_out
				(exchange_order_id, sales_invoice_item_id, variant_id, quantity, unit_price,
				 discount, bill_discount_share, tax_percent, tax_amount, total_price, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, exchangeID, si.ID, si.VariantID, item.Quantity, si.UnitPrice,
			itemDisc, lineBillDisc, si.TaxPercent, lineTax, lineReturn, item.Reason); err != nil {
			return "", fmt.Errorf("insert exchange_item_out: %w", err)
		}
		totalOutAmount += lineReturn
		totalGSTOut += lineTax
	}
	totalOutAmount = math.Round(totalOutAmount)
	totalGSTOut = round2(totalGSTOut)

	// ── 4. Process items_in ──────────────────────────────────────────────────
	type inCalc struct {
		variantID  string
		quantity   float64
		unitPrice  float64
		discount   float64
		taxPercent float64
		taxAmount  float64
		totalPrice float64
	}
	var insCalc []inCalc
	var subAmtIn, discAmtIn, gstAmtIn float64

	for _, item := range in.ItemsIn {
		if item.Quantity <= 0 {
			return "", fmt.Errorf("items_in quantity must be > 0")
		}

		unitPrice := item.UnitPrice
		if unitPrice <= 0 {
			if err := tx.QueryRow(`SELECT price FROM variants WHERE id = $1`, item.VariantID).Scan(&unitPrice); err != nil {
				return "", fmt.Errorf("fetch price for variant %s: %w", item.VariantID, err)
			}
		}

		lineTotal := item.Quantity * unitPrice
		itemDisc := item.Discount
		if item.DiscountType == "percent" {
			itemDisc = round2(lineTotal * item.Discount / 100)
		}
		itemDisc = round2(itemDisc)

		// GST slab: >2500 MRP → 18%, else 5% (same as billing)
		taxPercent := 5.0
		if item.ItemType == "MATERIAL" {
			if lineTotal > 2500 {
				taxPercent = 18
			}
		} else {
			if unitPrice > 2500 {
				taxPercent = 18
			}
		}

		taxable := lineTotal - itemDisc
		if taxable < 0 {
			taxable = 0
		}
		lineTax := round2(taxable * taxPercent / (100 + taxPercent))
		total := round2(lineTotal - itemDisc)

		ic := inCalc{
			variantID:  item.VariantID,
			quantity:   item.Quantity,
			unitPrice:  unitPrice,
			discount:   itemDisc,
			taxPercent: taxPercent,
			taxAmount:  lineTax,
			totalPrice: total,
		}
		insCalc = append(insCalc, ic)

		if _, err := tx.Exec(`
			INSERT INTO exchange_items_in
				(exchange_order_id, variant_id, quantity, unit_price, discount,
				 tax_percent, tax_amount, total_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, exchangeID, item.VariantID, item.Quantity, unitPrice, itemDisc,
			taxPercent, lineTax, total); err != nil {
			return "", fmt.Errorf("insert exchange_item_in: %w", err)
		}
		subAmtIn += lineTotal
		discAmtIn += itemDisc
		gstAmtIn += lineTax
	}
	subAmtIn = round2(subAmtIn)
	discAmtIn = round2(discAmtIn)
	gstAmtIn = round2(gstAmtIn)
	exactNet := round2(subAmtIn - discAmtIn)
	totalInAmount := math.Round(exactNet)
	roundOff := round2(totalInAmount - exactNet)

	// ── 5. Net amount and settlements validation ─────────────────────────────
	netAmount := round2(totalInAmount - totalOutAmount)

	var totalSettled float64
	for _, sett := range in.Settlements {
		totalSettled = round2(totalSettled + sett.Amount)
	}
	if netAmount > 0 && len(in.Settlements) == 0 {
		return "", fmt.Errorf("settlement required: customer must pay %.2f", netAmount)
	}
	if netAmount > 0 && math.Abs(totalSettled-netAmount) > 0.5 {
		return "", fmt.Errorf("settlement total %.2f does not match required %.2f", totalSettled, netAmount)
	}
	if netAmount < 0 && len(in.Settlements) == 0 {
		return "", fmt.Errorf("refund method required: store must refund %.2f", -netAmount)
	}

	settlementDirection := DirectionCollect
	if netAmount < 0 {
		settlementDirection = DirectionRefund
	}

	// ── 6. Create credit note (return_order, source='EXCHANGE') ─────────────
	// The original invoice is NOT modified.
	creditNoteNum := s.nextCreditNoteNumber(tx)
	var creditNoteID string
	if err := tx.QueryRow(`
		INSERT INTO return_orders
			(return_number, sales_invoice_id, branch_id, warehouse_id, customer_id,
			 status, refund_type, refund_amount, total_amount, gst_amount,
			 created_by, notes, source)
		VALUES ($1, $2, $3, $4, $5, 'COMPLETED', 'CREDIT', 0, $6, $7, $8, $9, 'EXCHANGE')
		RETURNING id
	`, creditNoteNum, inv.ID, inv.BranchID, inv.WarehouseID, inv.CustomerID,
		totalOutAmount, totalGSTOut, userID,
		"Credit note for exchange "+exchangeNumber).Scan(&creditNoteID); err != nil {
		return "", fmt.Errorf("create credit note: %w", err)
	}

	for _, oc := range outsCalc {
		if _, err := tx.Exec(`
			INSERT INTO return_items
				(return_order_id, sales_invoice_item_id, variant_id, quantity, unit_price,
				 discount, bill_discount_share, tax_percent, tax_amount, total_price, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, creditNoteID, oc.salesInvItemID, oc.variantID, oc.quantity, oc.unitPrice,
			oc.discount, oc.billDiscShare, oc.taxPercent, oc.taxAmount, oc.totalPrice, oc.reason); err != nil {
			return "", fmt.Errorf("insert return_item: %w", err)
		}
	}

	// ── 7. Stock IN for returned items ───────────────────────────────────────
	for _, oc := range outsCalc {
		if _, err := tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1, $2, $3, 'PRODUCT', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
		`, oc.variantID, inv.WarehouseID, oc.quantity); err != nil {
			return "", fmt.Errorf("stock in for returned item: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, 'EXCHANGE_IN', $4, 'COMPLETED')
		`, oc.variantID, inv.WarehouseID, oc.quantity, exchangeID); err != nil {
			return "", fmt.Errorf("stock movement for exchange in: %w", err)
		}
	}

	// ── 8. Create new sales order for items_in ───────────────────────────────
	var maxSO sql.NullString
	tx.QueryRow(`SELECT MAX(so_number) FROM sales_orders WHERE so_number LIKE 'SO%'`).Scan(&maxSO)
	soNext := 1
	if maxSO.Valid && len(maxSO.String) > 2 {
		fmt.Sscanf(maxSO.String[2:], "%d", &soNext)
		soNext++
	}
	soNumber := fmt.Sprintf("SO%05d", soNext)

	var newSalesOrderID string
	if err := tx.QueryRow(`
		INSERT INTO sales_orders
			(so_number, channel, branch_id, customer_id, warehouse_id,
			 created_by, order_date,
			 subtotal, tax_total, discount_total, bill_discount, grand_total,
			 status, payment_status, notes, salesperson_id)
		VALUES ($1, 'EXCHANGE', $2, $3, $4, $5, NOW(),
		        $6, $7, $8, 0, $9,
		        'CONFIRMED', 'PAID', $10, $11)
		RETURNING id
	`, soNumber, inv.BranchID, inv.CustomerID, inv.WarehouseID, userID,
		subAmtIn, gstAmtIn, discAmtIn, totalInAmount,
		"Exchange "+exchangeNumber, inv.SalespersonID).Scan(&newSalesOrderID); err != nil {
		return "", fmt.Errorf("create exchange sales order: %w", err)
	}

	// ── 9. Create new sales invoice for items_in ─────────────────────────────
	var maxInv sql.NullString
	tx.QueryRow(`SELECT MAX(invoice_number) FROM sales_invoices WHERE invoice_number LIKE 'INV%'`).Scan(&maxInv)
	invNext := 1
	if maxInv.Valid && len(maxInv.String) > 3 {
		fmt.Sscanf(maxInv.String[3:], "%d", &invNext)
		invNext++
	}
	invoiceNumber := fmt.Sprintf("INV%05d", invNext)

	var newInvoiceID string
	if err := tx.QueryRow(`
		INSERT INTO sales_invoices
			(sales_order_id, customer_id, warehouse_id, channel, branch_id,
			 invoice_number, invoice_date,
			 sub_amount, discount_amount, bill_discount, gst_amount, round_off,
			 net_amount, paid_amount, status, created_by, return_order_id)
		VALUES ($1, $2, $3, 'EXCHANGE', $4, $5, NOW(),
		        $6, $7, 0, $8, $9,
		        $10, $11, 'PAID', $12, $13)
		RETURNING id
	`, newSalesOrderID, inv.CustomerID, inv.WarehouseID, inv.BranchID,
		invoiceNumber,
		subAmtIn, discAmtIn, gstAmtIn, roundOff,
		totalInAmount, totalInAmount, userID, creditNoteID).Scan(&newInvoiceID); err != nil {
		return "", fmt.Errorf("create exchange invoice: %w", err)
	}

	for _, ic := range insCalc {
		if _, err := tx.Exec(`
			INSERT INTO sales_invoice_items
				(sales_invoice_id, variant_id, quantity, unit_price, discount,
				 tax_percent, tax_amount, total_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, newInvoiceID, ic.variantID, ic.quantity, ic.unitPrice, ic.discount,
			ic.taxPercent, ic.taxAmount, ic.totalPrice); err != nil {
			return "", fmt.Errorf("insert sales_invoice_item: %w", err)
		}
	}

	// ── 10. Sales payments on the new invoice ────────────────────────────────
	// Exchange credit applied (items returned by customer offset the new invoice)
	creditApplied := totalOutAmount
	if creditApplied > totalInAmount {
		creditApplied = totalInAmount
	}
	creditApplied = round2(creditApplied)

	if _, err := tx.Exec(`
		INSERT INTO sales_payments (sales_invoice_id, amount, payment_method, reference, paid_at)
		VALUES ($1, $2, 'EXCHANGE_CREDIT', $3, NOW())
	`, newInvoiceID, creditApplied, exchangeNumber); err != nil {
		return "", fmt.Errorf("record exchange credit payment: %w", err)
	}

	// If customer is paying the difference, record those payments too
	if netAmount > 0 {
		for _, sett := range in.Settlements {
			if _, err := tx.Exec(`
				INSERT INTO sales_payments (sales_invoice_id, amount, payment_method, reference, paid_at)
				VALUES ($1, $2, $3, $4, NOW())
			`, newInvoiceID, round2(sett.Amount), sett.PaymentMethod, sett.Reference); err != nil {
				return "", fmt.Errorf("record collect settlement: %w", err)
			}
		}
	}

	// ── 11. Stock OUT for new items ──────────────────────────────────────────
	for _, ic := range insCalc {
		res, err := tx.Exec(`
			UPDATE stocks
			SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id = $2 AND warehouse_id = $3 AND quantity >= $1
		`, ic.quantity, ic.variantID, inv.WarehouseID)
		if err != nil {
			return "", fmt.Errorf("deduct stock for exchange item in: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			var variantLabel string
			tx.QueryRow(`
				SELECT COALESCE(v.sku, p.name)
				FROM variants v JOIN products p ON p.id = v.product_id
				WHERE v.id = $1
			`, ic.variantID).Scan(&variantLabel)
			return "", fmt.Errorf("insufficient stock for %s", variantLabel)
		}
		if _, err := tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, from_warehouse_id, quantity, movement_type, reference, status, created_at)
			VALUES ($1, $2, $3, 'EXCHANGE_OUT', $4, 'COMPLETED', NOW())
		`, ic.variantID, inv.WarehouseID, ic.quantity, exchangeID); err != nil {
			return "", fmt.Errorf("stock movement for exchange out: %w", err)
		}
	}

	// ── 12. Exchange settlements ─────────────────────────────────────────────
	for _, sett := range in.Settlements {
		if _, err := tx.Exec(`
			INSERT INTO exchange_settlements
				(exchange_order_id, amount, payment_method, direction, reference)
			VALUES ($1, $2, $3, $4, $5)
		`, exchangeID, round2(sett.Amount), sett.PaymentMethod, settlementDirection, sett.Reference); err != nil {
			return "", fmt.Errorf("record exchange settlement: %w", err)
		}
	}

	// ── 13. Finalise exchange_order with computed totals ─────────────────────
	if _, err := tx.Exec(`
		UPDATE exchange_orders
		SET credit_note_id       = $1,
		    new_sales_invoice_id = $2,
		    items_out_total      = $3,
		    items_in_total       = $4,
		    net_amount           = $5,
		    completed_at         = NOW(),
		    updated_at           = NOW()
		WHERE id = $6
	`, creditNoteID, newInvoiceID, totalOutAmount, totalInAmount, netAmount, exchangeID); err != nil {
		return "", fmt.Errorf("finalise exchange order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit exchange: %w", err)
	}
	return exchangeID, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Cancel reverses a completed exchange:
//   - Cancels the credit note and new invoice
//   - Reverses all stock movements
//
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) Cancel(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exc struct {
		Status            string
		CreditNoteID      sql.NullString
		NewSalesInvoiceID sql.NullString
		WarehouseID       string
	}
	if err := tx.QueryRow(`
		SELECT status, credit_note_id, new_sales_invoice_id, warehouse_id
		FROM exchange_orders WHERE id = $1
	`, id).Scan(&exc.Status, &exc.CreditNoteID, &exc.NewSalesInvoiceID, &exc.WarehouseID); err != nil {
		return err
	}
	if exc.Status == StatusCancelled {
		return fmt.Errorf("exchange order is already cancelled")
	}

	// Load items_out (to reverse stock: those went IN, now go back OUT)
	outRows, err := tx.Query(`
		SELECT variant_id, quantity FROM exchange_items_out WHERE exchange_order_id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("load items_out for cancel: %w", err)
	}
	defer outRows.Close()
	type stockEntry struct {
		variantID string
		quantity  float64
	}
	var itemsOut []stockEntry
	for outRows.Next() {
		var e stockEntry
		if err := outRows.Scan(&e.variantID, &e.quantity); err != nil {
			return err
		}
		itemsOut = append(itemsOut, e)
	}
	outRows.Close()

	// Load items_in (to reverse stock: those went OUT, now come back IN)
	inRows, err := tx.Query(`
		SELECT variant_id, quantity FROM exchange_items_in WHERE exchange_order_id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("load items_in for cancel: %w", err)
	}
	defer inRows.Close()
	var itemsIn []stockEntry
	for inRows.Next() {
		var e stockEntry
		if err := inRows.Scan(&e.variantID, &e.quantity); err != nil {
			return err
		}
		itemsIn = append(itemsIn, e)
	}
	inRows.Close()

	// Cancel credit note
	if exc.CreditNoteID.Valid {
		if _, err := tx.Exec(`
			UPDATE return_orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1
		`, exc.CreditNoteID.String); err != nil {
			return fmt.Errorf("cancel credit note: %w", err)
		}
	}

	// Cancel new invoice
	if exc.NewSalesInvoiceID.Valid {
		if _, err := tx.Exec(`
			UPDATE sales_invoices SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1
		`, exc.NewSalesInvoiceID.String); err != nil {
			return fmt.Errorf("cancel new invoice: %w", err)
		}
	}

	// Reverse stock for items_out (they had come IN; now push them back OUT)
	for _, e := range itemsOut {
		if _, err := tx.Exec(`
			UPDATE stocks SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id = $2 AND warehouse_id = $3
		`, e.quantity, e.variantID, exc.WarehouseID); err != nil {
			return fmt.Errorf("reverse stock for item_out: %w", err)
		}
	}

	// Reverse stock for items_in (they had gone OUT; now bring them back IN)
	for _, e := range itemsIn {
		if _, err := tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1, $2, $3, 'PRODUCT', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
		`, e.variantID, exc.WarehouseID, e.quantity); err != nil {
			return fmt.Errorf("reverse stock for item_in: %w", err)
		}
	}

	// Cancel exchange order
	if _, err := tx.Exec(`
		UPDATE exchange_orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("cancel exchange order: %w", err)
	}

	return tx.Commit()
}

// ─────────────────────────────────────────────────────────────────────────────
// List returns a paginated list of exchange orders with summary info.
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) List(f ExchangeListFilter) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	idx := 1

	if f.BranchID != nil {
		where += fmt.Sprintf(" AND eo.branch_id = $%d", idx)
		args = append(args, *f.BranchID)
		idx++
	}
	if f.Status != "" {
		where += fmt.Sprintf(" AND eo.status = $%d", idx)
		args = append(args, f.Status)
		idx++
	}
	if f.Search != "" {
		where += fmt.Sprintf(
			" AND (eo.exchange_number ILIKE $%d OR c.name ILIKE $%d OR c.phone ILIKE $%d OR rn.return_number ILIKE $%d OR si.invoice_number ILIKE $%d)",
			idx, idx, idx, idx, idx)
		args = append(args, "%"+f.Search+"%")
		idx++
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM exchange_orders eo
		LEFT JOIN customers c ON c.id = eo.customer_id
		LEFT JOIN return_orders rn ON rn.id = eo.credit_note_id
		LEFT JOIN sales_invoices si ON si.id = eo.new_sales_invoice_id
		%s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQ := fmt.Sprintf(`
		SELECT
			eo.id, eo.exchange_number, eo.status,
			eo.items_out_total, eo.items_in_total, eo.net_amount,
			eo.created_at, eo.completed_at,
			COALESCE(c.name, '')   AS customer_name,
			COALESCE(c.phone, '')  AS customer_phone,
			COALESCE(b.name, '')   AS branch_name,
			COALESCE(rn.return_number, '')   AS credit_note_number,
			COALESCE(si.invoice_number, '')  AS new_invoice_number,
			eo.original_sales_invoice_id,
			COALESCE(orig.invoice_number, '') AS original_invoice_number
		FROM exchange_orders eo
		LEFT JOIN customers c ON c.id = eo.customer_id
		LEFT JOIN branches b ON b.id = eo.branch_id
		LEFT JOIN return_orders rn ON rn.id = eo.credit_note_id
		LEFT JOIN sales_invoices si ON si.id = eo.new_sales_invoice_id
		LEFT JOIN sales_invoices orig ON orig.id = eo.original_sales_invoice_id
		%s
		ORDER BY eo.created_at DESC`, where)

	if f.Limit > 0 {
		listQ += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
		args = append(args, f.Limit, f.Offset)
	}

	rows, err := s.db.Query(listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var (
			id, num, status                 string
			outTotal, inTotal, netAmt       float64
			createdAt                       interface{}
			completedAt                     interface{}
			custName, custPhone, branchName string
			cnNum, newInvNum                string
			origInvID, origInvNum           string
		)
		if err := rows.Scan(
			&id, &num, &status,
			&outTotal, &inTotal, &netAmt,
			&createdAt, &completedAt,
			&custName, &custPhone, &branchName,
			&cnNum, &newInvNum,
			&origInvID, &origInvNum,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, map[string]interface{}{
			"id":                      id,
			"exchange_number":         num,
			"status":                  status,
			"items_out_total":         outTotal,
			"items_in_total":          inTotal,
			"net_amount":              netAmt,
			"created_at":              createdAt,
			"completed_at":            completedAt,
			"customer_name":           custName,
			"customer_phone":          custPhone,
			"branch_name":             branchName,
			"credit_note_number":      cnNum,
			"new_invoice_number":      newInvNum,
			"original_invoice_id":     origInvID,
			"original_invoice_number": origInvNum,
		})
	}
	return result, total, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetByID returns full detail for a single exchange order.
// ─────────────────────────────────────────────────────────────────────────────
func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var exc struct {
		ID, ExchangeNumber, Status          string
		OrigInvID                           string
		CreditNoteID, NewInvID              sql.NullString
		BranchID, WarehouseID, CustomerID   sql.NullString
		ItemsOutTotal, ItemsInTotal, NetAmt float64
		Notes                               sql.NullString
		CreatedBy                           string
		CreatedAt                           interface{}
		CompletedAt                         interface{}
	}
	if err := s.db.QueryRow(`
		SELECT id, exchange_number, status,
		       original_sales_invoice_id, credit_note_id, new_sales_invoice_id,
		       branch_id, warehouse_id, customer_id,
		       items_out_total, items_in_total, net_amount,
		       notes, created_by, created_at, completed_at
		FROM exchange_orders WHERE id = $1
	`, id).Scan(
		&exc.ID, &exc.ExchangeNumber, &exc.Status,
		&exc.OrigInvID, &exc.CreditNoteID, &exc.NewInvID,
		&exc.BranchID, &exc.WarehouseID, &exc.CustomerID,
		&exc.ItemsOutTotal, &exc.ItemsInTotal, &exc.NetAmt,
		&exc.Notes, &exc.CreatedBy, &exc.CreatedAt, &exc.CompletedAt,
	); err != nil {
		return nil, err
	}

	// Supplementary invoice numbers
	var origInvNum, cnNum, newInvNum sql.NullString
	s.db.QueryRow(`SELECT invoice_number FROM sales_invoices WHERE id = $1`, exc.OrigInvID).Scan(&origInvNum)
	if exc.CreditNoteID.Valid {
		s.db.QueryRow(`SELECT return_number FROM return_orders WHERE id = $1`, exc.CreditNoteID.String).Scan(&cnNum)
	}
	if exc.NewInvID.Valid {
		s.db.QueryRow(`SELECT invoice_number FROM sales_invoices WHERE id = $1`, exc.NewInvID.String).Scan(&newInvNum)
	}

	// Items out
	outRows, err := s.db.Query(`
		SELECT eio.id, eio.sales_invoice_item_id, eio.variant_id, eio.quantity,
		       eio.unit_price, eio.discount, eio.bill_discount_share,
		       eio.tax_percent, eio.tax_amount, eio.total_price, eio.reason,
		       COALESCE(v.sku, p.name) AS variant_label,
		       COALESCE(v.name, '') AS variant_name,
		       COALESCE(p.name, '') AS product_name
		FROM exchange_items_out eio
		LEFT JOIN variants v ON v.id = eio.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		WHERE eio.exchange_order_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer outRows.Close()
	var itemsOut []map[string]interface{}
	for outRows.Next() {
		var (
			rowID, siItemID, varID, varLabel, varName, productName, reason string
			qty, up, disc, bdShare, txPct, txAmt, total                   float64
		)
		if err := outRows.Scan(&rowID, &siItemID, &varID, &qty, &up, &disc, &bdShare, &txPct, &txAmt, &total, &reason, &varLabel, &varName, &productName); err != nil {
			return nil, err
		}
		itemsOut = append(itemsOut, map[string]interface{}{
			"id": rowID, "sales_invoice_item_id": siItemID,
			"variant_id": varID, "variant_label": varLabel,
			"variant_name": varName, "product_name": productName,
			"quantity": qty, "unit_price": up, "discount": disc,
			"bill_discount_share": bdShare, "tax_percent": txPct,
			"tax_amount": txAmt, "total_price": total, "reason": reason,
		})
	}

	// Items in
	inRows, err := s.db.Query(`
		SELECT eii.id, eii.variant_id, eii.quantity,
		       eii.unit_price, eii.discount,
		       eii.tax_percent, eii.tax_amount, eii.total_price,
		       COALESCE(v.sku, p.name) AS variant_label,
		       COALESCE(v.name, '') AS variant_name,
		       COALESCE(p.name, '') AS product_name
		FROM exchange_items_in eii
		LEFT JOIN variants v ON v.id = eii.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		WHERE eii.exchange_order_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer inRows.Close()
	var itemsIn []map[string]interface{}
	for inRows.Next() {
		var (
			rowID, varID, varLabel, varName, productName string
			qty, up, disc, txPct, txAmt, total          float64
		)
		if err := inRows.Scan(&rowID, &varID, &qty, &up, &disc, &txPct, &txAmt, &total, &varLabel, &varName, &productName); err != nil {
			return nil, err
		}
		itemsIn = append(itemsIn, map[string]interface{}{
			"id": rowID, "variant_id": varID, "variant_label": varLabel,
			"variant_name": varName, "product_name": productName,
			"quantity": qty, "unit_price": up, "discount": disc,
			"tax_percent": txPct, "tax_amount": txAmt, "total_price": total,
		})
	}

	// Settlements
	settRows, err := s.db.Query(`
		SELECT id, amount, payment_method, direction, COALESCE(reference,''), settled_at
		FROM exchange_settlements WHERE exchange_order_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer settRows.Close()
	var settlements []map[string]interface{}
	for settRows.Next() {
		var rowID, method, direction, ref string
		var amt float64
		var settledAt interface{}
		if err := settRows.Scan(&rowID, &amt, &method, &direction, &ref, &settledAt); err != nil {
			return nil, err
		}
		settlements = append(settlements, map[string]interface{}{
			"id": rowID, "amount": amt, "payment_method": method,
			"direction": direction, "reference": ref, "settled_at": settledAt,
		})
	}

	return map[string]interface{}{
		"id":                      exc.ID,
		"exchange_number":         exc.ExchangeNumber,
		"status":                  exc.Status,
		"original_invoice_id":     exc.OrigInvID,
		"original_invoice_number": origInvNum.String,
		"credit_note_id":          exc.CreditNoteID.String,
		"credit_note_number":      cnNum.String,
		"new_sales_invoice_id":    exc.NewInvID.String,
		"new_invoice_number":      newInvNum.String,
		"branch_id":               exc.BranchID.String,
		"warehouse_id":            exc.WarehouseID.String,
		"customer_id":             exc.CustomerID.String,
		"items_out_total":         exc.ItemsOutTotal,
		"items_in_total":          exc.ItemsInTotal,
		"net_amount":              exc.NetAmt,
		"notes":                   exc.Notes.String,
		"created_by":              exc.CreatedBy,
		"created_at":              exc.CreatedAt,
		"completed_at":            exc.CompletedAt,
		"items_out":               itemsOut,
		"items_in":                itemsIn,
		"settlements":             settlements,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Number generators
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) nextExchangeNumber(tx *sql.Tx) string {
	var maxNo sql.NullString
	tx.QueryRow(`SELECT MAX(exchange_number) FROM exchange_orders WHERE exchange_number LIKE 'EXC%'`).Scan(&maxNo)
	next := 1
	if maxNo.Valid && len(maxNo.String) > 3 {
		fmt.Sscanf(maxNo.String[3:], "%d", &next)
		next++
	}
	return fmt.Sprintf("EXC%05d", next)
}

func (s *Store) nextCreditNoteNumber(tx *sql.Tx) string {
	var maxNo sql.NullString
	tx.QueryRow(`SELECT MAX(return_number) FROM return_orders WHERE return_number LIKE 'CN%'`).Scan(&maxNo)
	next := 1
	if maxNo.Valid && len(maxNo.String) > 2 {
		fmt.Sscanf(maxNo.String[2:], "%d", &next)
		next++
	}
	return fmt.Sprintf("CN%05d", next)
}

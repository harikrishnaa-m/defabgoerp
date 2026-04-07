package dashboard

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

// ════════════════════════════════════════════
// KPI Summary Cards
// ════════════════════════════════════════════

type KPISummary struct {
	TotalSales           float64 `json:"total_sales"`
	TotalPurchases       float64 `json:"total_purchases"`
	TotalReceivables     float64 `json:"total_receivables"`
	TotalPayables        float64 `json:"total_payables"`
	SalesInvoiceCount    int     `json:"sales_invoice_count"`
	PurchaseInvoiceCount int     `json:"purchase_invoice_count"`
	TotalCustomers       int     `json:"total_customers"`
	TotalProducts        int     `json:"total_products"`
	LowStockCount        int     `json:"low_stock_count"`
	PendingStockRequests int     `json:"pending_stock_requests"`
}

func (s *Store) GetKPISummary(from, to string) (*KPISummary, error) {
	kpi := &KPISummary{}
	dateFilter := ""
	args := []interface{}{}
	idx := 1

	if from != "" {
		dateFilter += fmt.Sprintf(" AND invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		dateFilter += fmt.Sprintf(" AND invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}

	// Sales totals
	salesQ := "SELECT COALESCE(COUNT(*),0), COALESCE(SUM(net_amount),0) FROM sales_invoices WHERE status != 'CANCELLED'" + dateFilter
	s.db.QueryRow(salesQ, args...).Scan(&kpi.SalesInvoiceCount, &kpi.TotalSales)

	// Purchase totals (reuse date filter with same args)
	purchaseQ := "SELECT COALESCE(COUNT(*),0), COALESCE(SUM(net_amount),0) FROM purchase_invoices WHERE status != 'CANCELLED'" + dateFilter
	s.db.QueryRow(purchaseQ, args...).Scan(&kpi.PurchaseInvoiceCount, &kpi.TotalPurchases)

	// Receivables (all-time outstanding)
	s.db.QueryRow("SELECT COALESCE(SUM(net_amount - paid_amount),0) FROM sales_invoices WHERE status IN ('UNPAID','PARTIAL')").Scan(&kpi.TotalReceivables)

	// Payables (all-time outstanding)
	s.db.QueryRow("SELECT COALESCE(SUM(net_amount - paid_amount),0) FROM purchase_invoices WHERE status IN ('PENDING','PARTIALLY_PAID')").Scan(&kpi.TotalPayables)

	// Counts
	s.db.QueryRow("SELECT COUNT(*) FROM customers WHERE is_active = TRUE").Scan(&kpi.TotalCustomers)
	s.db.QueryRow("SELECT COUNT(*) FROM products WHERE is_active = TRUE").Scan(&kpi.TotalProducts)

	// Low stock (variants with total quantity < 5 across all warehouses)
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT variant_id) FROM stocks
		WHERE stock_type = 'PRODUCT'
		GROUP BY variant_id HAVING SUM(quantity) < 5
	`).Scan(&kpi.LowStockCount)
	// Fix: count all low-stock variants
	s.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT variant_id FROM stocks
			WHERE stock_type = 'PRODUCT'
			GROUP BY variant_id HAVING SUM(quantity) < 5
		) sub
	`).Scan(&kpi.LowStockCount)

	// Pending stock requests
	s.db.QueryRow("SELECT COUNT(*) FROM stock_requests WHERE status = 'PENDING'").Scan(&kpi.PendingStockRequests)

	return kpi, nil
}

// ════════════════════════════════════════════
// Sales Trend (daily/monthly aggregation)
// ════════════════════════════════════════════

type TrendPoint struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

func (s *Store) SalesTrend(from, to, groupBy string) ([]TrendPoint, error) {
	dateTrunc := "day"
	format := "YYYY-MM-DD"
	if groupBy == "month" {
		dateTrunc = "month"
		format = "YYYY-MM"
	} else if groupBy == "week" {
		dateTrunc = "week"
		format = "IYYY-IW"
	}

	query := fmt.Sprintf(`
		SELECT TO_CHAR(DATE_TRUNC('%s', invoice_date), '%s') AS label,
		       COALESCE(SUM(net_amount), 0), COUNT(*)
		FROM sales_invoices
		WHERE status != 'CANCELLED'
	`, dateTrunc, format)

	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += " GROUP BY label ORDER BY label"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TrendPoint
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.Label, &p.Amount, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

// ════════════════════════════════════════════
// Purchase Trend
// ════════════════════════════════════════════

func (s *Store) PurchaseTrend(from, to, groupBy string) ([]TrendPoint, error) {
	dateTrunc := "day"
	format := "YYYY-MM-DD"
	if groupBy == "month" {
		dateTrunc = "month"
		format = "YYYY-MM"
	} else if groupBy == "week" {
		dateTrunc = "week"
		format = "IYYY-IW"
	}

	query := fmt.Sprintf(`
		SELECT TO_CHAR(DATE_TRUNC('%s', invoice_date), '%s') AS label,
		       COALESCE(SUM(net_amount), 0), COUNT(*)
		FROM purchase_invoices
		WHERE status != 'CANCELLED'
	`, dateTrunc, format)

	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += " GROUP BY label ORDER BY label"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TrendPoint
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.Label, &p.Amount, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

// ════════════════════════════════════════════
// Top Selling Products
// ════════════════════════════════════════════

type TopProduct struct {
	VariantID   string  `json:"variant_id"`
	VariantCode int     `json:"variant_code"`
	ProductName string  `json:"product_name"`
	VariantName string  `json:"variant_name"`
	SKU         string  `json:"sku"`
	TotalQty    int     `json:"total_qty"`
	TotalAmount float64 `json:"total_amount"`
}

func (s *Store) TopSellingProducts(from, to string, limit int) ([]TopProduct, error) {
	query := `
		SELECT sii.variant_id, v.variant_code, p.name, v.name, v.sku,
		       COALESCE(SUM(sii.quantity), 0),
		       COALESCE(SUM(sii.total_price), 0)
		FROM sales_invoice_items sii
		JOIN sales_invoices si ON si.id = sii.sales_invoice_id
		JOIN variants v ON v.id = sii.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE si.status != 'CANCELLED'
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND si.invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND si.invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += fmt.Sprintf(`
		GROUP BY sii.variant_id, v.variant_code, p.name, v.name, v.sku
		ORDER BY SUM(sii.quantity) DESC
		LIMIT $%d
	`, idx)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []TopProduct
	for rows.Next() {
		var tp TopProduct
		if err := rows.Scan(&tp.VariantID, &tp.VariantCode, &tp.ProductName, &tp.VariantName, &tp.SKU,
			&tp.TotalQty, &tp.TotalAmount); err != nil {
			return nil, err
		}
		products = append(products, tp)
	}
	return products, nil
}

// ════════════════════════════════════════════
// Top Customers
// ════════════════════════════════════════════

type TopCustomer struct {
	CustomerID   string  `json:"customer_id"`
	CustomerName string  `json:"customer_name"`
	Phone        string  `json:"phone"`
	InvoiceCount int     `json:"invoice_count"`
	TotalAmount  float64 `json:"total_amount"`
}

func (s *Store) TopCustomers(from, to string, limit int) ([]TopCustomer, error) {
	query := `
		SELECT c.id, c.name, COALESCE(c.phone,''), COUNT(si.id), COALESCE(SUM(si.net_amount), 0)
		FROM sales_invoices si
		JOIN customers c ON c.id = si.customer_id
		WHERE si.status != 'CANCELLED'
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND si.invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND si.invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += fmt.Sprintf(`
		GROUP BY c.id, c.name, c.phone
		ORDER BY SUM(si.net_amount) DESC
		LIMIT $%d
	`, idx)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []TopCustomer
	for rows.Next() {
		var tc TopCustomer
		if err := rows.Scan(&tc.CustomerID, &tc.CustomerName, &tc.Phone,
			&tc.InvoiceCount, &tc.TotalAmount); err != nil {
			return nil, err
		}
		customers = append(customers, tc)
	}
	return customers, nil
}

// ════════════════════════════════════════════
// Payment Method Breakdown (pie chart data)
// ════════════════════════════════════════════

type PaymentBreakdown struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

func (s *Store) SalesPaymentBreakdown(from, to string) ([]PaymentBreakdown, error) {
	query := `
		SELECT sp.payment_method, COALESCE(SUM(sp.amount), 0), COUNT(*)
		FROM sales_payments sp
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND sp.paid_at >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND sp.paid_at <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += " GROUP BY sp.payment_method ORDER BY SUM(sp.amount) DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PaymentBreakdown
	for rows.Next() {
		var pb PaymentBreakdown
		if err := rows.Scan(&pb.Method, &pb.Amount, &pb.Count); err != nil {
			return nil, err
		}
		items = append(items, pb)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Branch-wise Sales (bar chart data)
// ════════════════════════════════════════════

type BranchSales struct {
	BranchID   string  `json:"branch_id"`
	BranchName string  `json:"branch_name"`
	Amount     float64 `json:"amount"`
	Count      int     `json:"count"`
}

func (s *Store) BranchWiseSales(from, to string) ([]BranchSales, error) {
	query := `
		SELECT b.id, b.name, COALESCE(SUM(si.net_amount), 0), COUNT(si.id)
		FROM sales_invoices si
		JOIN branches b ON b.id = si.branch_id
		WHERE si.status != 'CANCELLED'
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND si.invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND si.invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += " GROUP BY b.id, b.name ORDER BY SUM(si.net_amount) DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []BranchSales
	for rows.Next() {
		var bs BranchSales
		if err := rows.Scan(&bs.BranchID, &bs.BranchName, &bs.Amount, &bs.Count); err != nil {
			return nil, err
		}
		items = append(items, bs)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Low Stock Alerts
// ════════════════════════════════════════════

type LowStockItem struct {
	VariantID   string  `json:"variant_id"`
	VariantCode int     `json:"variant_code"`
	ProductName string  `json:"product_name"`
	VariantName string  `json:"variant_name"`
	SKU         string  `json:"sku"`
	TotalQty    float64 `json:"total_qty"`
}

func (s *Store) LowStockAlerts(threshold float64, limit int) ([]LowStockItem, error) {
	rows, err := s.db.Query(`
		SELECT st.variant_id, v.variant_code, p.name, v.name, v.sku, SUM(st.quantity)
		FROM stocks st
		JOIN variants v ON v.id = st.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE st.stock_type = 'PRODUCT'
		GROUP BY st.variant_id, v.variant_code, p.name, v.name, v.sku
		HAVING SUM(st.quantity) < $1
		ORDER BY SUM(st.quantity) ASC
		LIMIT $2
	`, threshold, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []LowStockItem
	for rows.Next() {
		var li LowStockItem
		if err := rows.Scan(&li.VariantID, &li.VariantCode, &li.ProductName, &li.VariantName, &li.SKU, &li.TotalQty); err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Recent Activity Feed
// ════════════════════════════════════════════

type ActivityItem struct {
	Type      string    `json:"type"`
	Ref       string    `json:"ref"`
	RefID     string    `json:"ref_id"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) RecentActivity(limit int) ([]ActivityItem, error) {
	query := `
		(
			SELECT 'sales_invoice' AS type, invoice_number AS ref, id::text AS ref_id,
			       net_amount AS amount, status, created_at
			FROM sales_invoices ORDER BY created_at DESC LIMIT $1
		)
		UNION ALL
		(
			SELECT 'purchase_invoice', invoice_number, id::text,
			       net_amount, status, created_at
			FROM purchase_invoices ORDER BY created_at DESC LIMIT $1
		)
		UNION ALL
		(
			SELECT 'stock_request', 'SR-' || id::text, id::text,
			       0, status, created_at
			FROM stock_requests ORDER BY created_at DESC LIMIT $1
		)
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ActivityItem
	for rows.Next() {
		var a ActivityItem
		if err := rows.Scan(&a.Type, &a.Ref, &a.RefID, &a.Amount, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Outstanding Receivables & Payables
// ════════════════════════════════════════════

type OutstandingItem struct {
	ID            string  `json:"id"`
	InvoiceNumber string  `json:"invoice_number"`
	PartyName     string  `json:"party_name"`
	NetAmount     float64 `json:"net_amount"`
	PaidAmount    float64 `json:"paid_amount"`
	Outstanding   float64 `json:"outstanding"`
	InvoiceDate   string  `json:"invoice_date"`
}

func (s *Store) OutstandingReceivables(limit int) ([]OutstandingItem, error) {
	rows, err := s.db.Query(`
		SELECT si.id, si.invoice_number, COALESCE(c.name, ''),
		       si.net_amount, si.paid_amount, (si.net_amount - si.paid_amount),
		       si.invoice_date::text
		FROM sales_invoices si
		LEFT JOIN customers c ON c.id = si.customer_id
		WHERE si.status IN ('UNPAID', 'PARTIAL')
		ORDER BY (si.net_amount - si.paid_amount) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OutstandingItem
	for rows.Next() {
		var o OutstandingItem
		if err := rows.Scan(&o.ID, &o.InvoiceNumber, &o.PartyName,
			&o.NetAmount, &o.PaidAmount, &o.Outstanding, &o.InvoiceDate); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, nil
}

func (s *Store) OutstandingPayables(limit int) ([]OutstandingItem, error) {
	rows, err := s.db.Query(`
		SELECT pi.id, pi.invoice_number, COALESCE(sup.name, ''),
		       pi.net_amount, pi.paid_amount, (pi.net_amount - pi.paid_amount),
		       pi.invoice_date::text
		FROM purchase_invoices pi
		LEFT JOIN suppliers sup ON sup.id = pi.supplier_id
		WHERE pi.status IN ('PENDING', 'PARTIALLY_PAID')
		ORDER BY (pi.net_amount - pi.paid_amount) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OutstandingItem
	for rows.Next() {
		var o OutstandingItem
		if err := rows.Scan(&o.ID, &o.InvoiceNumber, &o.PartyName,
			&o.NetAmount, &o.PaidAmount, &o.Outstanding, &o.InvoiceDate); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Sales vs Purchase Comparison
// ════════════════════════════════════════════

type SalesVsPurchase struct {
	Label    string  `json:"label"`
	Sales    float64 `json:"sales"`
	Purchase float64 `json:"purchase"`
}

func (s *Store) SalesVsPurchaseTrend(from, to string) ([]SalesVsPurchase, error) {
	query := `
		WITH months AS (
			SELECT TO_CHAR(DATE_TRUNC('month', d), 'YYYY-MM') AS label,
			       DATE_TRUNC('month', d) AS m
			FROM generate_series($1::date, $2::date, '1 month'::interval) d
		),
		sales AS (
			SELECT TO_CHAR(DATE_TRUNC('month', invoice_date), 'YYYY-MM') AS label,
			       COALESCE(SUM(net_amount), 0) AS amount
			FROM sales_invoices WHERE status != 'CANCELLED'
			  AND invoice_date >= $1 AND invoice_date <= $2
			GROUP BY 1
		),
		purchases AS (
			SELECT TO_CHAR(DATE_TRUNC('month', invoice_date), 'YYYY-MM') AS label,
			       COALESCE(SUM(net_amount), 0) AS amount
			FROM purchase_invoices WHERE status != 'CANCELLED'
			  AND invoice_date >= $1 AND invoice_date <= $2
			GROUP BY 1
		)
		SELECT m.label, COALESCE(s.amount, 0), COALESCE(p.amount, 0)
		FROM months m
		LEFT JOIN sales s ON s.label = m.label
		LEFT JOIN purchases p ON p.label = m.label
		ORDER BY m.label
	`

	rows, err := s.db.Query(query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SalesVsPurchase
	for rows.Next() {
		var sv SalesVsPurchase
		if err := rows.Scan(&sv.Label, &sv.Sales, &sv.Purchase); err != nil {
			return nil, err
		}
		items = append(items, sv)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Category-wise Sales (pie/donut chart)
// ════════════════════════════════════════════

type CategorySales struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Amount       float64 `json:"amount"`
	Quantity     float64 `json:"quantity"`
}

func (s *Store) CategoryWiseSales(from, to string) ([]CategorySales, error) {
	query := `
		SELECT cat.id, cat.name, COALESCE(SUM(sii.total_price), 0), COALESCE(SUM(sii.quantity), 0)
		FROM sales_invoice_items sii
		JOIN sales_invoices si ON si.id = sii.sales_invoice_id
		JOIN variants v ON v.id = sii.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN categories cat ON cat.id = p.category_id
		WHERE si.status != 'CANCELLED'
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND si.invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND si.invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += " GROUP BY cat.id, cat.name ORDER BY SUM(sii.total_price) DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CategorySales
	for rows.Next() {
		var cs CategorySales
		if err := rows.Scan(&cs.CategoryID, &cs.CategoryName, &cs.Amount, &cs.Quantity); err != nil {
			return nil, err
		}
		items = append(items, cs)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Warehouse Stock Summary
// ════════════════════════════════════════════

type WarehouseStock struct {
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	BranchName    string  `json:"branch_name"`
	TotalVariants int     `json:"total_variants"`
	TotalQty      float64 `json:"total_qty"`
	StockValue    float64 `json:"stock_value"`
}

func (s *Store) WarehouseStockSummary() ([]WarehouseStock, error) {
	rows, err := s.db.Query(`
		SELECT w.id, w.name, COALESCE(b.name, ''),
		       COUNT(DISTINCT st.variant_id),
		       COALESCE(SUM(st.quantity), 0),
		       COALESCE(SUM(st.quantity * v.cost_price), 0)
		FROM warehouses w
		LEFT JOIN branches b ON b.id = w.branch_id
		LEFT JOIN stocks st ON st.warehouse_id = w.id AND st.stock_type = 'PRODUCT'
		LEFT JOIN variants v ON v.id = st.variant_id
		GROUP BY w.id, w.name, b.name
		ORDER BY SUM(st.quantity) DESC NULLS LAST
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WarehouseStock
	for rows.Next() {
		var ws WarehouseStock
		if err := rows.Scan(&ws.WarehouseID, &ws.WarehouseName, &ws.BranchName,
			&ws.TotalVariants, &ws.TotalQty, &ws.StockValue); err != nil {
			return nil, err
		}
		items = append(items, ws)
	}
	return items, nil
}

// ════════════════════════════════════════════
// Salesperson Performance
// ════════════════════════════════════════════

type SalespersonPerf struct {
	SalespersonID   string  `json:"salesperson_id"`
	SalespersonName string  `json:"salesperson_name"`
	InvoiceCount    int     `json:"invoice_count"`
	TotalSales      float64 `json:"total_sales"`
}

func (s *Store) SalespersonPerformance(from, to string, limit int) ([]SalespersonPerf, error) {
	query := `
		SELECT sp.id, sp.name, COUNT(si.id), COALESCE(SUM(si.net_amount), 0)
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN sales_persons sp ON sp.id = so.salesperson_id
		WHERE si.status != 'CANCELLED'
	`
	args := []interface{}{}
	idx := 1
	if from != "" {
		query += fmt.Sprintf(" AND si.invoice_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND si.invoice_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	query += fmt.Sprintf(`
		GROUP BY sp.id, sp.name
		ORDER BY SUM(si.net_amount) DESC
		LIMIT $%d
	`, idx)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SalespersonPerf
	for rows.Next() {
		var sp SalespersonPerf
		if err := rows.Scan(&sp.SalespersonID, &sp.SalespersonName, &sp.InvoiceCount, &sp.TotalSales); err != nil {
			return nil, err
		}
		items = append(items, sp)
	}
	return items, nil
}

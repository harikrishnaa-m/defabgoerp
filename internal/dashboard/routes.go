package dashboard

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	// KPI summary cards
	r.Get("/kpi", h.KPISummary)

	// Trend charts
	r.Get("/sales-trend", h.SalesTrend)
	r.Get("/purchase-trend", h.PurchaseTrend)
	r.Get("/sales-vs-purchase", h.SalesVsPurchase)

	// Rankings
	r.Get("/top-products", h.TopProducts)
	r.Get("/top-customers", h.TopCustomers)
	r.Get("/salesperson-performance", h.SalespersonPerformance)

	// Breakdowns (pie / donut)
	r.Get("/payment-breakdown", h.PaymentBreakdown)
	r.Get("/branch-sales", h.BranchSales)
	r.Get("/category-sales", h.CategorySales)

	// Inventory
	r.Get("/low-stock", h.LowStock)
	r.Get("/warehouse-stock", h.WarehouseStock)

	// Financials
	r.Get("/outstanding-receivables", h.OutstandingReceivables)
	r.Get("/outstanding-payables", h.OutstandingPayables)

	// Activity
	r.Get("/recent-activity", h.RecentActivity)
}

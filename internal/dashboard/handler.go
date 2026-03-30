package dashboard

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// GET /api/dashboard/kpi?from=&to=
func (h *Handler) KPISummary(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.store.GetKPISummary(from, to)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/sales-trend?from=&to=&group_by=day|week|month
func (h *Handler) SalesTrend(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	groupBy := c.Query("group_by", "day")
	data, err := h.store.SalesTrend(from, to, groupBy)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/purchase-trend?from=&to=&group_by=day|week|month
func (h *Handler) PurchaseTrend(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	groupBy := c.Query("group_by", "day")
	data, err := h.store.PurchaseTrend(from, to, groupBy)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/top-products?from=&to=&limit=10
func (h *Handler) TopProducts(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	limit := intQuery(c, "limit", 10)
	data, err := h.store.TopSellingProducts(from, to, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/top-customers?from=&to=&limit=10
func (h *Handler) TopCustomers(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	limit := intQuery(c, "limit", 10)
	data, err := h.store.TopCustomers(from, to, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/payment-breakdown?from=&to=
func (h *Handler) PaymentBreakdown(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.store.SalesPaymentBreakdown(from, to)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/branch-sales?from=&to=
func (h *Handler) BranchSales(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.store.BranchWiseSales(from, to)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/low-stock?threshold=5&limit=20
func (h *Handler) LowStock(c *fiber.Ctx) error {
	threshold := floatQuery(c, "threshold", 5)
	limit := intQuery(c, "limit", 20)
	data, err := h.store.LowStockAlerts(threshold, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/recent-activity?limit=15
func (h *Handler) RecentActivity(c *fiber.Ctx) error {
	limit := intQuery(c, "limit", 15)
	data, err := h.store.RecentActivity(limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/outstanding-receivables?limit=20
func (h *Handler) OutstandingReceivables(c *fiber.Ctx) error {
	limit := intQuery(c, "limit", 20)
	data, err := h.store.OutstandingReceivables(limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/outstanding-payables?limit=20
func (h *Handler) OutstandingPayables(c *fiber.Ctx) error {
	limit := intQuery(c, "limit", 20)
	data, err := h.store.OutstandingPayables(limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/sales-vs-purchase?from=2025-04-01&to=2026-03-31
func (h *Handler) SalesVsPurchase(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		return c.Status(400).JSON(fiber.Map{"error": "from and to are required"})
	}
	data, err := h.store.SalesVsPurchaseTrend(from, to)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/category-sales?from=&to=
func (h *Handler) CategorySales(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.store.CategoryWiseSales(from, to)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/warehouse-stock
func (h *Handler) WarehouseStock(c *fiber.Ctx) error {
	data, err := h.store.WarehouseStockSummary()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// GET /api/dashboard/salesperson-performance?from=&to=&limit=10
func (h *Handler) SalespersonPerformance(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	limit := intQuery(c, "limit", 10)
	data, err := h.store.SalespersonPerformance(from, to, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": data})
}

// ── helpers ──

func intQuery(c *fiber.Ctx, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func floatQuery(c *fiber.Ctx, key string, def float64) float64 {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n < 0 {
		return def
	}
	return n
}

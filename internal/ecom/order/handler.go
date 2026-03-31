package order

import (
	"math"
	"strings"

	ecomMw "defab-erp/internal/ecom/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(s *Store) *Handler {
	return &Handler{store: s}
}

// ── Customer-facing endpoints ───────────────────────────────

// POST /ecom/orders/checkout
func (h *Handler) Checkout(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)

	var in CheckoutInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if in.PaymentMethod == "" {
		in.PaymentMethod = "COD"
	}
	in.PaymentMethod = strings.ToUpper(in.PaymentMethod)

	result, err := h.store.Checkout(cust.ID, in)
	if err != nil {
		msg := err.Error()
		if msg == "cart is empty" || msg == "address not found" {
			return c.Status(400).JSON(fiber.Map{"error": msg})
		}
		return c.Status(500).JSON(fiber.Map{"error": "checkout failed"})
	}

	return c.Status(201).JSON(result)
}

// GET /ecom/orders
func (h *Handler) ListOrders(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	orders, total, err := h.store.ListOrders(cust.ID, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch orders"})
	}
	if orders == nil {
		orders = []map[string]interface{}{}
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        orders,
	})
}

// GET /ecom/orders/:id
func (h *Handler) GetOrder(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	orderID := c.Params("id")

	order, err := h.store.GetOrder(cust.ID, orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "order not found"})
	}
	return c.JSON(order)
}

// POST /ecom/orders/:id/cancel
func (h *Handler) CancelOrder(c *fiber.Ctx) error {
	cust := c.Locals("ecom_customer").(*ecomMw.EcomCustomer)
	orderID := c.Params("id")

	if err := h.store.CancelOrder(cust.ID, orderID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "order cancelled"})
}

// ── Admin endpoints (ERP staff) ─────────────────────────────

// GET /ecom/admin/orders
func (h *Handler) AdminListOrders(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	status := strings.ToUpper(c.Query("status"))

	orders, total, err := h.store.AdminListOrders(status, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch orders"})
	}
	if orders == nil {
		orders = []map[string]interface{}{}
	}

	return c.JSON(fiber.Map{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		"data":        orders,
	})
}

// GET /ecom/admin/orders/:id
func (h *Handler) AdminGetOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")

	order, err := h.store.AdminGetOrder(orderID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "order not found"})
	}
	return c.JSON(order)
}

// PATCH /ecom/admin/orders/:id/status
func (h *Handler) AdminUpdateStatus(c *fiber.Ctx) error {
	orderID := c.Params("id")

	var in UpdateStatusInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	in.Status = strings.ToUpper(in.Status)

	valid := map[string]bool{
		"PENDING": true, "CONFIRMED": true, "PROCESSING": true,
		"SHIPPED": true, "DELIVERED": true, "CANCELLED": true, "RETURNED": true,
	}
	if !valid[in.Status] {
		return c.Status(400).JSON(fiber.Map{"error": "invalid status"})
	}

	if err := h.store.AdminUpdateStatus(orderID, in.Status); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "status updated", "status": in.Status})
}

// PATCH /ecom/admin/orders/:id/payment
func (h *Handler) AdminUpdatePayment(c *fiber.Ctx) error {
	orderID := c.Params("id")

	var in UpdatePaymentInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON"})
	}
	in.PaymentStatus = strings.ToUpper(in.PaymentStatus)

	valid := map[string]bool{"UNPAID": true, "PAID": true, "REFUNDED": true}
	if !valid[in.PaymentStatus] {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payment_status"})
	}

	if err := h.store.AdminUpdatePayment(orderID, in.PaymentStatus, in.PaymentRef); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "payment updated"})
}

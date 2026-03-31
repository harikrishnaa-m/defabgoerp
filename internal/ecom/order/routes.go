package order

import "github.com/gofiber/fiber/v2"

// RegisterCustomerRoutes registers authenticated ecom customer order routes.
func RegisterCustomerRoutes(r fiber.Router, h *Handler) {
	r.Post("/checkout", h.Checkout)
	r.Get("/", h.ListOrders)
	r.Get("/:id", h.GetOrder)
	r.Post("/:id/cancel", h.CancelOrder)
}

// RegisterAdminRoutes registers ERP-admin order management routes.
func RegisterAdminRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.AdminListOrders)
	r.Get("/:id", h.AdminGetOrder)
	r.Patch("/:id/status", h.AdminUpdateStatus)
	r.Patch("/:id/payment", h.AdminUpdatePayment)
}

package purchaseinvoice

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/purchase-invoices")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/:id", h.GetByID)
	g.Post("/:id/payments", h.RecordPayment)
	g.Delete("/:id", h.Cancel)

	// Supplier payments endpoints
	sp := r.Group("/supplier-payments")
	sp.Get("/", h.ListAllPayments)
	sp.Get("/outstanding", h.OutstandingSummary)
	sp.Get("/supplier/:supplierId", h.ListPaymentsBySupplier)
}

package purchaseinvoice

import "github.com/gofiber/fiber/v2"

func RegisterInvoiceRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/:id", h.GetByID)
	r.Post("/:id/payments", h.RecordPayment)
	r.Delete("/:id", h.Cancel)
}

func RegisterPaymentRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.ListAllPayments)
	r.Get("/outstanding", h.OutstandingSummary)
	r.Get("/supplier/:supplierId", h.ListPaymentsBySupplier)
}

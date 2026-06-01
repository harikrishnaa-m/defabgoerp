package salesinvoice

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
	r.Get("/by-number/:invoiceNumber", h.GetByInvoiceNumber)
	r.Get("/:id", h.GetByID)
	r.Patch("/:id/salesperson", h.UpdateSalesperson)
	r.Delete("/:id", h.CancelInvoice)
}

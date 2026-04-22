package migration

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/dry-run", h.DryRun)
	r.Post("/import-xlsx", h.ImportXlsx)
	r.Post("/import-sales", h.ImportSales)
}

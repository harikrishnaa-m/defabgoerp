package migration

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/dry-run", h.DryRun)
	r.Post("/import-xlsx", h.ImportXlsx)
	r.Post("/upsert-xlsx", h.UpsertXlsx)
	r.Post("/reprice-from-xlsx", h.RepriceFromXlsx)
	r.Post("/import-sales", h.ImportSales)
	r.Post("/import-vyttila-stock", h.ImportVyttilaStock)
}

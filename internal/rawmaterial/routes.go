package rawmaterial

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.ListAll)
	r.Get("/warehouse/:warehouseId", h.ListByWarehouse)
	r.Get("/movements", h.ListMovements)
	r.Get("/movements/branch", h.MovementsByBranch)
	r.Get("/movements/:id", h.MovementByID)
	r.Get("/branch", h.StocksByBranch)
	r.Post("/adjust", h.AdjustStock)
}

package rawmaterial

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(api fiber.Router, h *Handler) {
	g := api.Group("/raw-material-stocks")
	g.Get("/", h.ListAll)
	g.Get("/warehouse/:warehouseId", h.ListByWarehouse)
	g.Get("/movements", h.ListMovements)
	g.Get("/movements/branch", h.MovementsByBranch)
	g.Get("/movements/:id", h.MovementByID)
	g.Get("/branch", h.StocksByBranch)
	g.Post("/adjust", h.AdjustStock)
}

package goodsreceipt

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/goods-receipts")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/po/:poId", h.ListByPO)
	g.Get("/:id", h.GetByID)
}

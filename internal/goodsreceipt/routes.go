package goodsreceipt

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/po/:poId", h.ListByPO)
	r.Get("/:id", h.GetByID)
	r.Delete("/:id", h.Cancel)
}

//tested

package warehouse

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/warehouses")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Patch("/:id", h.Update)
	g.Delete("/:id", h.Delete)
	g.Get("/:id", h.GetByID)
}

//tested

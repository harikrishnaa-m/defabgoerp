package category

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/categories")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/:id", h.Get)
	g.Patch("/:id", h.Update)

	g.Patch("/:id/deactivate", h.Deactivate)
	g.Patch("/:id/activate", h.Activate)
}

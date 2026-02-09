package branch

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/branches")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Patch("/:id", h.Update)

}

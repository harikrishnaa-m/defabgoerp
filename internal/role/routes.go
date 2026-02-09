package role

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/roles")

	g.Post("/", h.Create)
	g.Get("/", h.List)
}

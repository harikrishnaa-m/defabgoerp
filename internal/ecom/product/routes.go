package product

import "github.com/gofiber/fiber/v2"

// RegisterRoutes registers public (no auth) product catalog routes.
func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
	r.Get("/categories", h.Categories)
	r.Get("/:id", h.GetByID)
}

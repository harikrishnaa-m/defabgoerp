package productdescription

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/product-descriptions")

	g.Post("/", h.Create)
	g.Get("/:productId", h.Get)
	g.Patch("/:productId", h.Update)
}
package productdescription

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/:productId", h.Get)
	r.Patch("/:productId", h.Update)
}

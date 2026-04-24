package onlinestock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Set)
	r.Get("/", h.List)
	r.Patch("/:variant_id", h.Update)
}

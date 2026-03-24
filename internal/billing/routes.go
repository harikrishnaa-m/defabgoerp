package billing

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/lookup", h.Lookup)
	r.Get("/cache", h.CacheStatus)
	r.Get("/:id", h.GetByID)
}

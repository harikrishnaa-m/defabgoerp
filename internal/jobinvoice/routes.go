package jobinvoice

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
	r.Post("/backfill", h.Backfill)
	r.Get("/:id", h.GetByID)
}

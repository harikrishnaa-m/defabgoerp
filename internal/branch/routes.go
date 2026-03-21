package branch

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Patch("/:id", h.Update)
	r.Get("/:id", h.GetByID)
}

//tested

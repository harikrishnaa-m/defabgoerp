package branch

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Patch("/:id", h.Update)
	r.Get("/:id", h.GetByID)
}

func RegisterListRoute(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
}

//tested

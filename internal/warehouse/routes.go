package warehouse

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Patch("/:id", h.Update)
	r.Delete("/:id", h.Delete)
	r.Get("/:id", h.GetByID)
}

func RegisterListRoute(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
}

//tested

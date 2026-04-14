package salesperson

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/:id", h.GetByID)
	r.Patch("/:id", h.Update)
	r.Patch("/:id/activate", h.Activate)
	r.Patch("/:id/deactivate", h.Deactivate)
}

//tested

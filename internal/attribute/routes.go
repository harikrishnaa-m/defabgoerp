package attribute

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/attributes")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Patch("/:id", h.Update)
	g.Patch("/:id/deactivate", h.Deactivate)
	g.Patch("/:id/activate", h.Activate)

	g.Post("/values", h.CreateValue)
	g.Get("/:id/values", h.ListValues)
	g.Patch("/values/:id", h.UpdateValue)
	g.Patch("/values/:id/deactivate", h.DeactivateValue)
	g.Patch("/values/:id/activate", h.ActivateValue)
}

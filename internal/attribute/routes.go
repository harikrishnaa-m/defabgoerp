package attribute

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Patch("/:id", h.Update)
	r.Patch("/:id/deactivate", h.Deactivate)
	r.Patch("/:id/activate", h.Activate)

	r.Post("/values", h.CreateValue)
	r.Get("/:id/values", h.ListValues)
	r.Patch("/values/:id", h.UpdateValue)
	r.Patch("/values/:id/deactivate", h.DeactivateValue)
	r.Patch("/values/:id/activate", h.ActivateValue)
}

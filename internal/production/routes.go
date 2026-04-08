package production

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/:id", h.GetByID)
	r.Post("/:id/status", h.PushStatus)
	r.Post("/:id/complete", h.Complete)
	r.Delete("/:id", h.Cancel)
}

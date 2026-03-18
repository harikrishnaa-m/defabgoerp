package stockrequest

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/stock-requests")

	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Get("/:id", h.GetByID)

	g.Patch("/:id/decision", h.Approve)
	g.Delete("/:id", h.Cancel)

	g.Post("/:id/dispatch", h.Dispatch)
}

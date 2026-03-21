package stockrequest

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/branch", h.ByBranch)
	r.Get("/:id", h.GetByID)

	r.Patch("/:id/decision", h.Approve)
	r.Delete("/:id", h.Cancel)

	r.Post("/:id/dispatch", h.Dispatch)
	r.Post("/:id/receive", h.Receive)
}

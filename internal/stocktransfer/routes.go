package stocktransfer

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	g := r.Group("/stock-transfers")
	g.Post("/", h.Create)

	g.Post("/transfers/:id/receive", h.Receive)
}

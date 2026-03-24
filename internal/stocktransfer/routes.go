package stocktransfer

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Post("/", h.Create)

	r.Post("/transfers/:id/receive", h.Receive)
}

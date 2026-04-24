package wishlist

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	r.Get("/", h.List)
	r.Post("/items", h.Add)
	r.Delete("/items/:product_id", h.Remove)
}

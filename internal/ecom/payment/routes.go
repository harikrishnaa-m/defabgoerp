package payment

import "github.com/gofiber/fiber/v2"

// RegisterAuthRoutes registers routes that require ecom JWT auth.
func RegisterAuthRoutes(ecomProtected fiber.Router, h *Handler) {
	ecomProtected.Post("/payments/initiate", h.Initiate)
	ecomProtected.Get("/payments/:order_id/verify", h.Verify)
}

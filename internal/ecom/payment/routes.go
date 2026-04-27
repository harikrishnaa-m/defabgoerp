package payment

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(ecomProtected fiber.Router, public fiber.Router, h *Handler) {
	// Authenticated: customer initiates payment for their order
	ecomProtected.Post("/payments/initiate", h.Initiate)

	// Public: Cashfree webhook — verified by signature, not JWT
	public.Post("/payments/webhook", h.Webhook)
}
